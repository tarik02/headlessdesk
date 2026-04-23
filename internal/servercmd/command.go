package servercmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"libfreerdp-golang-poc/internal/control"
	"libfreerdp-golang-poc/internal/freerdp"
	"libfreerdp-golang-poc/internal/healthapi"
	"libfreerdp-golang-poc/internal/httpapi"
	"libfreerdp-golang-poc/internal/mcpapi"
)

type config struct {
	ListenAddr    string `mapstructure:"listen_addr"`
	MCPPath       string `mapstructure:"mcp_path"`
	EnableHTTPAPI bool   `mapstructure:"enable_http_api"`
	EnableMCPAPI  bool   `mapstructure:"enable_mcp_api"`

	Host           string `mapstructure:"host"`
	Port           uint   `mapstructure:"port"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	Domain         string `mapstructure:"domain"`
	Width          uint   `mapstructure:"width"`
	Height         uint   `mapstructure:"height"`
	KeyboardLayout uint   `mapstructure:"keyboard_layout"`
	Insecure       bool   `mapstructure:"insecure"`
	GraphicsMode   string `mapstructure:"graphics_mode"`
}

func New() *cobra.Command {
	var configPath string
	v := viper.New()

	cmd := &cobra.Command{
		Use:           "server",
		Short:         "RDP screenshot and control server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVar(&configPath, "config", "", "path to a YAML, TOML, or JSON config file")
	persistentFlags.String("host", "", "RDP server hostname or IP")
	persistentFlags.Uint("port", 3389, "RDP server port")
	persistentFlags.String("username", "", "RDP username")
	persistentFlags.String("password", "", "RDP password")
	persistentFlags.String("domain", "", "optional RDP domain")
	persistentFlags.Uint("width", 1280, "requested remote desktop width")
	persistentFlags.Uint("height", 720, "requested remote desktop height")
	persistentFlags.Uint("keyboard-layout", 0x0409, "RDP keyboard layout id")
	persistentFlags.Bool("insecure", false, "accept unknown or changed server certificates")
	persistentFlags.String("graphics-mode", "auto", "graphics transport: auto, gfx, or bitmap")

	mustBindFlag(v, "host", persistentFlags.Lookup("host"))
	mustBindFlag(v, "port", persistentFlags.Lookup("port"))
	mustBindFlag(v, "username", persistentFlags.Lookup("username"))
	mustBindFlag(v, "password", persistentFlags.Lookup("password"))
	mustBindFlag(v, "domain", persistentFlags.Lookup("domain"))
	mustBindFlag(v, "width", persistentFlags.Lookup("width"))
	mustBindFlag(v, "height", persistentFlags.Lookup("height"))
	mustBindFlag(v, "keyboard_layout", persistentFlags.Lookup("keyboard-layout"))
	mustBindFlag(v, "insecure", persistentFlags.Lookup("insecure"))
	mustBindFlag(v, "graphics_mode", persistentFlags.Lookup("graphics-mode"))

	cmd.AddCommand(newServeCommand(v, &configPath))
	cmd.AddCommand(newStdioMCPCommand(v, &configPath))
	return cmd
}

func Execute() error {
	return New().Execute()
}

func newServeCommand(v *viper.Viper, configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server with health, REST, and optional MCP endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(v, *configPath)
			if err != nil {
				return err
			}
			if err := validateRDPConfig(cfg); err != nil {
				return err
			}
			if err := validateServeConfig(cfg); err != nil {
				return err
			}
			return runServe(cfg)
		},
	}

	flags := cmd.Flags()
	flags.String("listen-addr", ":8080", "HTTP listen address")
	flags.String("mcp-path", "/mcp", "HTTP path for the MCP streamable endpoint")
	flags.Bool("enable-http-api", true, "enable the REST screenshot and input API")
	flags.Bool("enable-mcp-api", true, "enable the MCP streamable HTTP API")

	mustBindFlag(v, "listen_addr", flags.Lookup("listen-addr"))
	mustBindFlag(v, "mcp_path", flags.Lookup("mcp-path"))
	mustBindFlag(v, "enable_http_api", flags.Lookup("enable-http-api"))
	mustBindFlag(v, "enable_mcp_api", flags.Lookup("enable-mcp-api"))

	return cmd
}

func newStdioMCPCommand(v *viper.Viper, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stdio-mcp",
		Short: "Run the MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(v, *configPath)
			if err != nil {
				return err
			}
			if err := validateRDPConfig(cfg); err != nil {
				return err
			}
			return runStdioMCP(cfg)
		},
	}
}

func loadConfig(v *viper.Viper, configPath string) (config, error) {
	v.SetEnvPrefix("RDP")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func validateRDPConfig(cfg config) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("host is required")
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return errors.New("username is required")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return errors.New("password is required")
	}
	if cfg.Port > math.MaxUint16 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}
	if cfg.Width == 0 {
		return errors.New("width must be greater than zero")
	}
	if cfg.Height == 0 {
		return errors.New("height must be greater than zero")
	}
	if cfg.Width > math.MaxUint32 || cfg.Height > math.MaxUint32 {
		return fmt.Errorf("invalid dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.GraphicsMode)) {
	case "", "auto", "avc", "h264", "gfx", "graphics", "bitmap", "legacy":
	default:
		return fmt.Errorf("invalid graphics-mode: %q", cfg.GraphicsMode)
	}
	return nil
}

func validateServeConfig(cfg config) error {
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return errors.New("listen-addr is required")
	}
	if cfg.EnableMCPAPI && !strings.HasPrefix(cfg.MCPPath, "/") {
		return errors.New("mcp-path must start with '/'")
	}
	return nil
}

func runServe(cfg config) error {
	session, err := startSession(cfg)
	if err != nil {
		return err
	}
	defer closeSession(session)

	service := control.NewService(session)
	router := gin.Default()

	healthapi.RegisterRoutes(router, service)
	if cfg.EnableHTTPAPI {
		httpapi.RegisterRoutes(router, service)
	}
	if cfg.EnableMCPAPI {
		router.Any(cfg.MCPPath, gin.WrapH(mcpapi.NewHTTPHandler(service)))
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("http server listening on %s", cfg.ListenAddr)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- fmt.Errorf("http server failed: %w", err)
			return
		}
		serverErrCh <- nil
	}()

	select {
	case err := <-serverErrCh:
		return err
	case <-ctx.Done():
		return shutdownHTTPServer(server)
	case <-session.Done():
		if err := session.Err(); err != nil {
			log.Printf("rdp session ended: %v", err)
			if shutdownErr := shutdownHTTPServer(server); shutdownErr != nil {
				return errors.Join(err, shutdownErr)
			}
			return err
		}
		return shutdownHTTPServer(server)
	}
}

func runStdioMCP(cfg config) error {
	session, err := startSession(cfg)
	if err != nil {
		return err
	}
	defer closeSession(session)
	logSessionEnd(session)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return mcpapi.RunStdio(ctx, control.NewService(session))
}

func startSession(cfg config) (*freerdp.Session, error) {
	session, err := freerdp.StartSession(freerdp.Config{
		Host:               cfg.Host,
		Port:               uint16(cfg.Port),
		Username:           cfg.Username,
		Password:           cfg.Password,
		Domain:             cfg.Domain,
		DesktopWidth:       uint32(cfg.Width),
		DesktopHeight:      uint32(cfg.Height),
		KeyboardLayout:     uint32(cfg.KeyboardLayout),
		InsecureSkipVerify: cfg.Insecure,
		GraphicsMode:       cfg.GraphicsMode,
	})
	if err != nil {
		return nil, fmt.Errorf("start RDP session: %w", err)
	}
	return session, nil
}

func closeSession(session *freerdp.Session) {
	select {
	case <-session.Done():
		_ = session.Close()
		return
	default:
	}
	if err := session.Close(); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("session close error: %v", err)
	}
}

func shutdownHTTPServer(server *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http shutdown error: %w", err)
	}
	return nil
}

func logSessionEnd(session *freerdp.Session) {
	go func() {
		<-session.Done()
		if err := session.Err(); err != nil {
			log.Printf("rdp session ended: %v", err)
			return
		}
		log.Printf("rdp session ended")
	}()
}

func mustBindFlag(v *viper.Viper, key string, flag *pflag.Flag) {
	if flag == nil {
		panic("flag must not be nil")
	}
	if err := v.BindPFlag(key, flag); err != nil {
		panic(err)
	}
}
