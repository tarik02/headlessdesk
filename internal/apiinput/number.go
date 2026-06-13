package apiinput

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"strings"
)

type Number struct {
	raw json.RawMessage
}

func (n *Number) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		n.raw = nil
		return nil
	}
	n.raw = append(n.raw[:0], data...)
	return nil
}

func (n Number) Set() bool {
	return len(n.raw) > 0
}

func (n Number) Float64(field string) (float64, error) {
	if !n.Set() {
		return 0, nil
	}
	value, err := parseValue(n.raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	return value, nil
}

func (n Number) Int(field string) (int, error) {
	value, err := n.Float64(field)
	if err != nil {
		return 0, err
	}
	return roundInt(field, value)
}

func RequiredFloat64(field string, value *Number) (float64, error) {
	if value == nil || !value.Set() {
		return 0, fmt.Errorf("%s is required", field)
	}
	return value.Float64(field)
}

func OptionalInt(field string, value *Number) (*int, error) {
	if value == nil || !value.Set() {
		return nil, nil
	}
	result, err := value.Int(field)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func parseValue(raw json.RawMessage) (float64, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		expr := strings.TrimSpace(text)
		if expr == "" {
			return 0, errors.New("expression is empty")
		}
		return evalExpression(expr)
	}

	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, errors.New("must be a number or arithmetic expression string")
	}
	if !isFinite(value) {
		return 0, errors.New("must be finite")
	}
	return value, nil
}

func evalExpression(expr string) (float64, error) {
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression %q", expr)
	}

	if err := validateExpression(parsed); err != nil {
		return 0, err
	}

	result, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression %q: %w", expr, err)
	}
	if result.Value == nil {
		return 0, errors.New("expression must be constant")
	}
	value, _ := constant.Float64Val(result.Value)
	if !isFinite(value) {
		return 0, errors.New("expression result must be finite")
	}
	return value, nil
}

func validateExpression(expr ast.Expr) error {
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.INT && node.Kind != token.FLOAT {
			return fmt.Errorf("unsupported literal %q", node.Value)
		}
		return nil
	case *ast.ParenExpr:
		return validateExpression(node.X)
	case *ast.UnaryExpr:
		if node.Op != token.ADD && node.Op != token.SUB {
			return fmt.Errorf("unsupported unary operator %q", node.Op)
		}
		return validateExpression(node.X)
	case *ast.BinaryExpr:
		if node.Op != token.ADD && node.Op != token.SUB && node.Op != token.MUL && node.Op != token.QUO {
			return fmt.Errorf("unsupported binary operator %q", node.Op)
		}
		if err := validateExpression(node.X); err != nil {
			return err
		}
		return validateExpression(node.Y)
	default:
		return errors.New("only numeric literals, parentheses, +, -, *, and / are supported")
	}
}

func roundInt(field string, value float64) (int, error) {
	if !isFinite(value) {
		return 0, fmt.Errorf("%s: must be finite", field)
	}
	rounded := math.Round(value)
	maxInt := float64(int(^uint(0) >> 1))
	minInt := -maxInt - 1
	if rounded < minInt || rounded > maxInt {
		return 0, fmt.Errorf("%s: rounded value is outside int range", field)
	}
	return int(rounded), nil
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}
