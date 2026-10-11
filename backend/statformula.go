package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Stat formulas are stored as plain text the user builds out of blocks in
// Settings, for example:
//
//	10 + [vit] * 10 + [level] * 2
//
// Grammar (usual precedence, ^ binds tightest and to the right):
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/") unary }
//	unary   = "-" unary | power
//	power   = primary [ "^" unary ]
//	primary = number | "[" name "]" | name "(" [ expr { "," expr } ] ")" | "(" expr ")"
//
// [name] reads a value: a stat by its key, or one of formulaInputs. An empty
// formula is 0. Evaluation never fails: dividing by zero, the square root of
// a negative number and the like give 0, so a formula can't break a sheet.

// formulaInputs are the values every formula can read besides stats. [base]
// is the number typed on the sheet for the stat being calculated; the bonus
// inputs (bonusInputs) are what each kind of asset gives to it.
var formulaInputs = append([]string{"level", "age", "height", "weight", "base"}, bonusInputNames()...)

// bonusInputs maps a bonus block to the stat_modifiers source_type it reads:
// [race_bonus] is the bonus the character's race gives the stat being
// calculated. A formula that reads one takes that bonus over, so it is no
// longer added on its own (see statSystem.calculate).
var bonusInputs = map[string]string{
	"class_bonus":          "class",
	"subclass_bonus":       "subclass",
	"specialization_bonus": "specialization",
	"race_bonus":           "race",
	"body_type_bonus":      "body_type",
	"gear_bonus":           "item",
}

func bonusInputNames() []string {
	return []string{"class_bonus", "subclass_bonus", "specialization_bonus", "race_bonus", "body_type_bonus", "gear_bonus"}
}

func isFormulaInput(name string) bool {
	for _, n := range formulaInputs {
		if n == name {
			return true
		}
	}
	return false
}

// perStatInput: an input that only means something for one stat ([base] and
// the bonuses), so the points formula can't read it.
func perStatInput(name string) bool {
	_, bonus := bonusInputs[name]
	return name == "base" || bonus
}

// formulaFuncs are the functions a formula can call, with the fewest and
// most arguments each takes (-1: no limit).
var formulaFuncs = map[string][2]int{
	"min":   {2, -1},
	"max":   {2, -1},
	"round": {1, 2}, // round(x) or round(x, digits)
	"floor": {1, 1},
	"ceil":  {1, 1},
	"abs":   {1, 1},
	"sqrt":  {1, 1},
	"clamp": {3, 3}, // clamp(x, lowest, highest)
	// curve(x, x1, y1, x2, y2, ...) draws straight lines between the points
	// and stays flat before the first and after the last.
	"curve": {5, -1},
}

// formulaError says what is wrong with a formula. Code is stable (the
// interface translates it); Detail names the offending piece.
type formulaError struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

func (e *formulaError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

type formulaNode interface {
	eval(env func(string) float64) float64
}

type numNode float64
type refNode string
type negNode struct{ x formulaNode }
type binNode struct {
	op   byte
	a, b formulaNode
}
type callNode struct {
	fn   string
	args []formulaNode
}

func (n numNode) eval(func(string) float64) float64     { return float64(n) }
func (n refNode) eval(env func(string) float64) float64 { return env(string(n)) }
func (n negNode) eval(env func(string) float64) float64 { return -n.x.eval(env) }

func (n binNode) eval(env func(string) float64) float64 {
	a, b := n.a.eval(env), n.b.eval(env)
	switch n.op {
	case '+':
		return a + b
	case '-':
		return a - b
	case '*':
		return a * b
	case '/':
		if b == 0 {
			return 0
		}
		return a / b
	default: // '^'
		return finite(math.Pow(a, b))
	}
}

func (n callNode) eval(env func(string) float64) float64 {
	v := make([]float64, len(n.args))
	for i, a := range n.args {
		v[i] = a.eval(env)
	}
	switch n.fn {
	case "min":
		m := v[0]
		for _, x := range v[1:] {
			m = math.Min(m, x)
		}
		return m
	case "max":
		m := v[0]
		for _, x := range v[1:] {
			m = math.Max(m, x)
		}
		return m
	case "round":
		if len(v) == 1 {
			return math.Round(v[0])
		}
		p := math.Pow(10, math.Round(clamp(v[1], -6, 6)))
		return math.Round(v[0]*p) / p
	case "floor":
		return math.Floor(v[0])
	case "ceil":
		return math.Ceil(v[0])
	case "abs":
		return math.Abs(v[0])
	case "sqrt":
		if v[0] < 0 {
			return 0
		}
		return math.Sqrt(v[0])
	case "clamp":
		lo, hi := v[1], v[2]
		if lo > hi {
			lo, hi = hi, lo
		}
		return clamp(v[0], lo, hi)
	default: // "curve"
		return curve(v[0], v[1:])
	}
}

// curve interpolates x along points (x1, y1, x2, y2, ...). A lone trailing
// number (an odd count) is ignored.
func curve(x float64, pts []float64) float64 {
	n := len(pts) / 2
	if x <= pts[0] {
		return pts[1]
	}
	if x >= pts[2*(n-1)] {
		return pts[2*(n-1)+1]
	}
	for i := 0; i < n-1; i++ {
		x0, y0, x1, y1 := pts[2*i], pts[2*i+1], pts[2*i+2], pts[2*i+3]
		if x >= x0 && x <= x1 {
			if x1 == x0 {
				return y1
			}
			return y0 + (x-x0)/(x1-x0)*(y1-y0)
		}
	}
	return pts[2*(n-1)+1] // points out of order; settle on the last
}

func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// formula is a parsed formula and every [name] it reads.
type formula struct {
	root formulaNode
	refs []string
}

func (f *formula) eval(env func(string) float64) float64 {
	if f == nil || f.root == nil {
		return 0
	}
	return finite(f.root.eval(env))
}

func (f *formula) reads(name string) bool {
	if f == nil {
		return false
	}
	for _, r := range f.refs {
		if r == name {
			return true
		}
	}
	return false
}

type formulaToken struct {
	kind byte // 'n' number, 'r' [ref], 'f' function name, or the symbol itself
	text string
	num  float64
}

func tokenizeFormula(src string) ([]formulaToken, error) {
	var out []formulaToken
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c >= '0' && c <= '9' || c == '.':
			j := i
			for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '.') {
				j++
			}
			v, err := strconv.ParseFloat(src[i:j], 64)
			if err != nil {
				return nil, &formulaError{Code: "bad_number", Detail: src[i:j]}
			}
			out = append(out, formulaToken{kind: 'n', text: src[i:j], num: v})
			i = j
		case c == '[':
			j := strings.IndexByte(src[i:], ']')
			if j < 0 {
				return nil, &formulaError{Code: "syntax", Detail: "["}
			}
			name := strings.TrimSpace(src[i+1 : i+j])
			if !validStatKey(name) {
				return nil, &formulaError{Code: "unknown_value", Detail: name}
			}
			out = append(out, formulaToken{kind: 'r', text: name})
			i += j + 1
		case c >= 'a' && c <= 'z':
			j := i
			for j < len(src) && (src[j] >= 'a' && src[j] <= 'z' || src[j] >= '0' && src[j] <= '9' || src[j] == '_') {
				j++
			}
			out = append(out, formulaToken{kind: 'f', text: src[i:j]})
			i = j
		case strings.IndexByte("+-*/^(),", c) >= 0:
			out = append(out, formulaToken{kind: c, text: string(c)})
			i++
		default:
			return nil, &formulaError{Code: "syntax", Detail: string(c)}
		}
	}
	return out, nil
}

type formulaParser struct {
	toks []formulaToken
	pos  int
	refs []string
}

func (p *formulaParser) peek() byte {
	if p.pos < len(p.toks) {
		return p.toks[p.pos].kind
	}
	return 0
}

func (p *formulaParser) expr() (formulaNode, error) {
	left, err := p.term()
	if err != nil {
		return nil, err
	}
	for k := p.peek(); k == '+' || k == '-'; k = p.peek() {
		p.pos++
		right, err := p.term()
		if err != nil {
			return nil, err
		}
		left = binNode{op: k, a: left, b: right}
	}
	return left, nil
}

func (p *formulaParser) term() (formulaNode, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for k := p.peek(); k == '*' || k == '/'; k = p.peek() {
		p.pos++
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		left = binNode{op: k, a: left, b: right}
	}
	return left, nil
}

func (p *formulaParser) unary() (formulaNode, error) {
	if p.peek() == '-' {
		p.pos++
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return negNode{x}, nil
	}
	base, err := p.primary()
	if err != nil {
		return nil, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.unary()
		if err != nil {
			return nil, err
		}
		return binNode{op: '^', a: base, b: exp}, nil
	}
	return base, nil
}

func (p *formulaParser) primary() (formulaNode, error) {
	if p.pos >= len(p.toks) {
		return nil, &formulaError{Code: "incomplete"}
	}
	t := p.toks[p.pos]
	p.pos++
	switch t.kind {
	case 'n':
		return numNode(t.num), nil
	case 'r':
		p.refs = append(p.refs, t.text)
		return refNode(t.text), nil
	case '(':
		x, err := p.expr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ')' {
			return nil, &formulaError{Code: "unclosed"}
		}
		p.pos++
		return x, nil
	case 'f':
		limits, ok := formulaFuncs[t.text]
		if !ok {
			return nil, &formulaError{Code: "unknown_function", Detail: t.text}
		}
		if p.peek() != '(' {
			return nil, &formulaError{Code: "syntax", Detail: t.text}
		}
		p.pos++
		var args []formulaNode
		if p.peek() != ')' {
			for {
				a, err := p.expr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if p.peek() != ',' {
					break
				}
				p.pos++
			}
		}
		if p.peek() != ')' {
			return nil, &formulaError{Code: "unclosed"}
		}
		p.pos++
		if len(args) < limits[0] || limits[1] >= 0 && len(args) > limits[1] {
			return nil, &formulaError{Code: "arg_count", Detail: t.text}
		}
		return callNode{fn: t.text, args: args}, nil
	default:
		return nil, &formulaError{Code: "syntax", Detail: t.text}
	}
}

// parseFormula reads a formula. It checks the syntax only; whether every
// [name] exists is up to the caller, which knows the stats.
func parseFormula(src string) (*formula, error) {
	toks, err := tokenizeFormula(src)
	if err != nil {
		return nil, err
	}
	if len(toks) == 0 {
		return &formula{}, nil
	}
	p := &formulaParser{toks: toks}
	root, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		if p.toks[p.pos].kind == ')' {
			return nil, &formulaError{Code: "unopened"}
		}
		return nil, &formulaError{Code: "syntax", Detail: p.toks[p.pos].text}
	}
	return &formula{root: root, refs: p.refs}, nil
}

// validStatKey: lowercase letters, digits and underscores, starting with a
// letter. Keys are stored in formulas and on characters, so they never change
// once made.
func validStatKey(k string) bool {
	if k == "" || len(k) > 40 || k[0] < 'a' || k[0] > 'z' {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func mustFormula(src string) *formula {
	f, err := parseFormula(src)
	if err != nil {
		panic(fmt.Sprintf("formula %q: %v", src, err))
	}
	return f
}
