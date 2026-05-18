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

	"headlessdesk/internal/control"
	"headlessdesk/internal/desktop"
	"headlessdesk/internal/freerdp"
	"headlessdesk/internal/healthapi"
	"headlessdesk/internal/httpapi"
	"headlessdesk/internal/mcpapi"
	"headlessdesk/internal/vnc"
)

type config struct {
	Server  serverConfig  `mapstructure:"server"`
	Session sessionConfig `mapstructure:"session"`
	RDP     rdpConfig     `mapstructure:"rdp"`
	VNC     vncConfig     `mapstructure:"vnc"`
}

type serverConfig struct {
	ListenAddr    string `mapstructure:"listen_addr"`
	MCPPath       string `mapstructure:"mcp_path"`
	EnableHTTPAPI bool   `mapstructure:"enable_http_api"`
	EnableMCPAPI  bool   `mapstructure:"enable_mcp_api"`
}

type sessionConfig struct {
	Protocol string `mapstructure:"protocol"`
	Host     string `mapstructure:"host"`
	Port     uint   `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Width    uint   `mapstructure:"width"`
	Height   uint   `mapstructure:"height"`
	Insecure bool   `mapstructure:"insecure"`
}

type rdpConfig struct {
	Domain         string `mapstructure:"domain"`
	KeyboardLayout uint   `mapstructure:"keyboard_layout"`
	GraphicsMode   string `mapstructure:"graphics_mode"`
}

type vncConfig struct {
	Shared   bool `mapstructure:"shared"`
	ViewOnly bool `mapstructure:"view_only"`
}

func New() *cobra.Command {
	var configPath string
	v := viper.New()

	cmd := &cobra.Command{
		Use:           "headlessdesk",
		Short:         "Remote desktop screenshot and control server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	setDefaults(v)

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVar(&configPath, "config", "", "path to a YAML, TOML, or JSON config file")
	persistentFlags.String("protocol", "rdp", "remote desktop protocol: rdp or vnc")
	persistentFlags.String("remote-host", "", "remote desktop server hostname or IP")
	persistentFlags.Uint("remote-port", 0, "remote desktop server port; defaults to the selected protocol")
	persistentFlags.String("username", "", "remote desktop username")
	persistentFlags.String("password", "", "remote desktop password")
	persistentFlags.Uint("width", 1280, "requested remote desktop width")
	persistentFlags.Uint("height", 720, "requested remote desktop height")
	persistentFlags.Bool("insecure", false, "accept unknown or changed server certificates where supported")
	persistentFlags.String("rdp-domain", "", "optional RDP domain")
	persistentFlags.Uint("rdp-keyboard-layout", 0x0409, "RDP keyboard layout id")
	persistentFlags.String("rdp-graphics-mode", "auto", "RDP graphics transport: auto, gfx, or bitmap")
	persistentFlags.Bool("vnc-shared", true, "request a shared VNC session")
	persistentFlags.Bool("vnc-view-only", false, "connect to VNC without sending input events")

	mustBindFlag(v, "session.protocol", persistentFlags.Lookup("protocol"))
	mustBindFlag(v, "session.host", persistentFlags.Lookup("remote-host"))
	mustBindFlag(v, "session.port", persistentFlags.Lookup("remote-port"))
	mustBindFlag(v, "session.username", persistentFlags.Lookup("username"))
	mustBindFlag(v, "session.password", persistentFlags.Lookup("password"))
	mustBindFlag(v, "session.width", persistentFlags.Lookup("width"))
	mustBindFlag(v, "session.height", persistentFlags.Lookup("height"))
	mustBindFlag(v, "session.insecure", persistentFlags.Lookup("insecure"))
	mustBindFlag(v, "rdp.domain", persistentFlags.Lookup("rdp-domain"))
	mustBindFlag(v, "rdp.keyboard_layout", persistentFlags.Lookup("rdp-keyboard-layout"))
	mustBindFlag(v, "rdp.graphics_mode", persistentFlags.Lookup("rdp-graphics-mode"))
	mustBindFlag(v, "vnc.shared", persistentFlags.Lookup("vnc-shared"))
	mustBindFlag(v, "vnc.view_only", persistentFlags.Lookup("vnc-view-only"))

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
			if err := validateSessionConfig(cfg); err != nil {
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

	mustBindFlag(v, "server.listen_addr", flags.Lookup("listen-addr"))
	mustBindFlag(v, "server.mcp_path", flags.Lookup("mcp-path"))
	mustBindFlag(v, "server.enable_http_api", flags.Lookup("enable-http-api"))
	mustBindFlag(v, "server.enable_mcp_api", flags.Lookup("enable-mcp-api"))

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
			if err := validateSessionConfig(cfg); err != nil {
				return err
			}
			return runStdioMCP(cfg)
		},
	}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("session.protocol", "rdp")
	v.SetDefault("session.width", 1280)
	v.SetDefault("session.height", 720)
	v.SetDefault("rdp.keyboard_layout", 0x0409)
	v.SetDefault("rdp.graphics_mode", "auto")
	v.SetDefault("vnc.shared", true)
}

func loadConfig(v *viper.Viper, configPath string) (config, error) {
	v.SetEnvPrefix("HEADLESSDESK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
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

func validateSessionConfig(cfg config) error {
	protocol := normalizedProtocol(cfg)
	if protocol == "" {
		return errors.New("session.protocol is required")
	}
	if protocol != "rdp" && protocol != "vnc" {
		return fmt.Errorf("invalid session.protocol: %q", cfg.Session.Protocol)
	}
	if strings.TrimSpace(cfg.Session.Host) == "" {
		return errors.New("session.host is required")
	}
	if protocol == "rdp" && strings.TrimSpace(cfg.Session.Username) == "" {
		return errors.New("session.username is required for rdp")
	}
	if protocol == "rdp" && strings.TrimSpace(cfg.Session.Password) == "" {
		return errors.New("session.password is required for rdp")
	}
	if cfg.Session.Port > math.MaxUint16 {
		return fmt.Errorf("invalid session.port: %d", cfg.Session.Port)
	}
	if cfg.Session.Width == 0 {
		return errors.New("session.width must be greater than zero")
	}
	if cfg.Session.Height == 0 {
		return errors.New("session.height must be greater than zero")
	}
	if cfg.Session.Width > math.MaxUint32 || cfg.Session.Height > math.MaxUint32 {
		return fmt.Errorf("invalid session dimensions: %dx%d", cfg.Session.Width, cfg.Session.Height)
	}
	if protocol == "rdp" {
		switch strings.ToLower(strings.TrimSpace(cfg.RDP.GraphicsMode)) {
		case "", "auto", "avc", "h264", "gfx", "graphics", "bitmap", "legacy":
		default:
			return fmt.Errorf("invalid rdp.graphics_mode: %q", cfg.RDP.GraphicsMode)
		}
	}
	if protocol == "vnc" && cfg.VNC.ViewOnly {
		return errors.New("vnc.view_only cannot be true because control APIs require input")
	}
	return nil
}

func validateServeConfig(cfg config) error {
	if strings.TrimSpace(cfg.Server.ListenAddr) == "" {
		return errors.New("server.listen_addr is required")
	}
	if cfg.Server.EnableMCPAPI && !strings.HasPrefix(cfg.Server.MCPPath, "/") {
		return errors.New("server.mcp_path must start with '/'")
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
	if cfg.Server.EnableHTTPAPI {
		httpapi.RegisterRoutes(router, service)
	}
	if cfg.Server.EnableMCPAPI {
		router.Any(cfg.Server.MCPPath, gin.WrapH(mcpapi.NewHTTPHandler(service)))
	}

	server := &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("http server listening on %s", cfg.Server.ListenAddr)
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
			log.Printf("%s session ended: %v", normalizedProtocol(cfg), err)
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
	logSessionEnd(normalizedProtocol(cfg), session)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return mcpapi.RunStdio(ctx, control.NewService(session))
}

func startSession(cfg config) (desktop.Session, error) {
	switch normalizedProtocol(cfg) {
	case "rdp":
		session, err := freerdp.StartSession(freerdp.Config{
			Host:               cfg.Session.Host,
			Port:               sessionPort(cfg, 3389),
			Username:           cfg.Session.Username,
			Password:           cfg.Session.Password,
			Domain:             cfg.RDP.Domain,
			DesktopWidth:       uint32(cfg.Session.Width),
			DesktopHeight:      uint32(cfg.Session.Height),
			KeyboardLayout:     uint32(cfg.RDP.KeyboardLayout),
			InsecureSkipVerify: cfg.Session.Insecure,
			GraphicsMode:       cfg.RDP.GraphicsMode,
		})
		if err != nil {
			return nil, fmt.Errorf("start RDP session: %w", err)
		}
		return session, nil
	case "vnc":
		session, err := vnc.StartSession(vnc.Config{
			Host:          cfg.Session.Host,
			Port:          sessionPort(cfg, 5900),
			Username:      cfg.Session.Username,
			Password:      cfg.Session.Password,
			DesktopWidth:  uint32(cfg.Session.Width),
			DesktopHeight: uint32(cfg.Session.Height),
			Shared:        cfg.VNC.Shared,
			ViewOnly:      cfg.VNC.ViewOnly,
		})
		if err != nil {
			return nil, fmt.Errorf("start VNC session: %w", err)
		}
		return session, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Session.Protocol)
	}
}

func normalizedProtocol(cfg config) string {
	return strings.ToLower(strings.TrimSpace(cfg.Session.Protocol))
}

func sessionPort(cfg config, fallback uint16) uint16 {
	if cfg.Session.Port == 0 {
		return fallback
	}
	return uint16(cfg.Session.Port)
}

func closeSession(session desktop.Session) {
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

func logSessionEnd(protocol string, session desktop.Session) {
	go func() {
		<-session.Done()
		if err := session.Err(); err != nil {
			log.Printf("%s session ended: %v", protocol, err)
			return
		}
		log.Printf("%s session ended", protocol)
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
