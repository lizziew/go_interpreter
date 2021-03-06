package evaluator

import (
	"fmt"
	"github.com/fatih/color"
	"go_interpreter/ast"
	"go_interpreter/object"
)

var PRINT_EVAL = false

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	if PRINT_EVAL {
		color.Green("EVAL %T: evaluator.Eval(%s)", node, node.String())
	}
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.Boolean:
		return evalBoolean(node.Value)
	case *ast.Prefix:
		value := Eval(node.Value, env)
		if isError(value) {
			return value
		}
		return evalPrefix(node.Operator, value)
	case *ast.Infix:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}

		return evalInfix(left, node.Operator, right)
	case *ast.If:
		return evalIf(node, env)
	case *ast.ReturnStatement:
		value := Eval(node.Value, env)
		if isError(value) {
			return value
		}
		return &object.Return{Value: value}
	case *ast.LetStatement:
		value := Eval(node.Value, env)
		if isError(value) {
			return value
		}

		env.Set(node.Name.Value, value)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.Function:
		return &object.Function{node.Parameters, node.Body, env}
	case *ast.Call:
		f := Eval(node.Function, env)
		if isError(f) {
			return f
		}

		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		return evalFunction(f, args)
	case *ast.String:
		return &object.String{node.Value}
	case *ast.Array:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}

		return &object.Array{Elements: elements}
	case *ast.Index:
		array := Eval(node.Array, env)
		if isError(array) {
			return array
		}

		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}

		return evalIndex(array, index)
	case *ast.Hash:
		return evalHash(node, env)
	}

	return nil
}

// Helper method for evaluating expressions
func evalExpressions(args []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, a := range args {
		value := Eval(a, env)
		if isError(value) {
			return []object.Object{value}
		}

		result = append(result, value)
	}

	return result
}

// Helper method for evaluating function
func evalFunction(fobj object.Object, args []object.Object) object.Object {
	switch f := fobj.(type) {
	case *object.Function:
		outerEnv := extendEnv(f, args)
		value := Eval(f.Body, outerEnv)

		result, ok := value.(*object.Return)
		if ok {
			return result.Value
		} else {
			return value
		}
	case *object.BuiltIn:
		return f.Function(args...)
	default:
		return NewError("not a function: %s", f.Type())
	}
}

// Helper method for extending environment for evaluating function
func extendEnv(f *object.Function, args []object.Object) *object.Environment {
	innerEnv := object.BuildInnerEnvironment(f.Env)

	// Bind arguments to parameter names
	for i, p := range f.Parameters {
		innerEnv.Set(p.Value, args[i])
	}

	return innerEnv
}

// Helper method for evaluating boolean
func evalBoolean(expression bool) object.Object {
	if expression {
		return TRUE
	} else {
		return FALSE
	}
}

// Helper method for evaluating statements in a program
func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.Return:
			return result.Value
		case *object.Error:
			return result
		}
	}
	return result
}

// Helper method for evaluating statements in a block statement
func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil &&
			(result.Type() == object.RETURN_OBJECT || result.Type() == object.ERROR_OBJECT) {
			return result
		}
	}

	return result
}

// Helper method for evaluating prefix
func evalPrefix(operator string, expression object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangPrefix(expression)
	case "-":
		return evalMinusPrefix(expression)
	default:
		return NewError("unknown operator: %s%s", operator, expression.Type())
	}
}

// Helper method for evaluating prefix !
func evalBangPrefix(expression object.Object) object.Object {
	switch expression {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

// Helper method for evaluating prefix -
func evalMinusPrefix(expression object.Object) object.Object {
	if expression.Type() != object.INTEGER_OBJECT {
		return NewError("unknown operator: -%s", expression.Type())
	}

	result := expression.(*object.Integer).Value
	return &object.Integer{Value: -result}
}

// Helper method for evaluating infix
func evalInfix(left object.Object, operator string, right object.Object) object.Object {
	switch {
	case left.Type() != right.Type():
		return NewError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	case left.Type() == object.INTEGER_OBJECT && right.Type() == object.INTEGER_OBJECT:
		leftValue := left.(*object.Integer).Value
		rightValue := right.(*object.Integer).Value
		return evalIntegerInfix(leftValue, operator, rightValue)
	case left.Type() == object.STRING_OBJECT && right.Type() == object.STRING_OBJECT:
		leftValue := left.(*object.String).Value
		rightValue := right.(*object.String).Value
		return evalStringInfix(leftValue, operator, rightValue)
	case operator == "==":
		return evalBoolean(left == right)
	case operator == "!=":
		return evalBoolean(left != right)
	default:
		return NewError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

// Helper method for evaluating string infix
func evalStringInfix(left string, operator string, right string) object.Object {
	if operator != "+" {
		return NewError("unknown operator: %s %s %s",
			object.STRING_OBJECT, operator, object.STRING_OBJECT)
	}

	return &object.String{left + right}
}

// Helper method for evaluating integer infix
func evalIntegerInfix(left int64, operator string, right int64) object.Object {
	switch operator {
	case "+":
		return &object.Integer{Value: left + right}
	case "-":
		return &object.Integer{Value: left - right}
	case "*":
		return &object.Integer{Value: left * right}
	case "/":
		return &object.Integer{Value: left / right}
	case "<":
		return evalBoolean(left < right)
	case ">":
		return evalBoolean(left > right)
	case "==":
		return evalBoolean(left == right)
	case "!=":
		return evalBoolean(left != right)
	default:
		return NewError("unknown operator: %s %s %s", left, operator, right)
	}
}

// Helper method for evaluating if
func evalIf(i *ast.If, env *object.Environment) object.Object {
	condition := Eval(i.Condition, env)

	if isError(condition) {
		return condition
	}

	if isTrue(condition) {
		return Eval(i.Consequence, env)
	} else if i.Alternative != nil {
		return Eval(i.Alternative, env)
	} else {
		return NULL
	}
}

// Helper method for defining what is true
func isTrue(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

// Helper method for evaluating identifiers
func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	value, ok := env.Get(node.Value)
	if ok {
		return value
	}

	builtin, ok := builtins[node.Value]
	if ok {
		return builtin
	}

	return NewError("identifier not found: " + node.Value)
}

// Helper method for reporting errors
func NewError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

// Helper method for stopping errors from bubbling up
func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJECT
	}
	return false
}

// Helper method for evaluating index expressions
func evalIndex(accessObj object.Object, indexObj object.Object) object.Object {
	switch {
	case accessObj.Type() == object.ARRAY_OBJECT && indexObj.Type() == object.INTEGER_OBJECT:
		array := accessObj.(*object.Array)
		index := indexObj.(*object.Integer).Value

		end := int64(len(array.Elements) - 1)
		if index < 0 || index > end {
			return NULL
		} else {
			return array.Elements[index]
		}
	case accessObj.Type() == object.HASH_OBJECT:
		hash := accessObj.(*object.Hash)
		key, ok := indexObj.(object.Hashable)
		if !ok {
			return NewError("unusable as hash key")
		}

		pair, ok := hash.Pairs[key.HashKey()]
		if !ok {
			return NULL
		} else {
			return pair.Value
		}
	default:
		return NewError("index operator not supported: %s", accessObj.Type())
	}
}

// Helper method for evaluating hash expressions
func evalHash(node *ast.Hash, env *object.Environment) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	for k, v := range node.Pairs {
		// Get key
		key := Eval(k, env)
		if isError(key) {
			return key
		}

		// Get hashed key
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return NewError("ununsable as hash key")
		}

		// Get value
		value := Eval(v, env)
		if isError(value) {
			return value
		}

		pairs[hashKey.HashKey()] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: pairs}
}
