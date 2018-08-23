package vm

import (
	"fmt"
	"go_interpreter/bytecode"
	"go_interpreter/compiler"
	"go_interpreter/object"
)

const stackCapacity = 2048
const GlobalCapacity = 65536 // Upper limit on number of global bindings

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants    []object.Object       // Constants generated by compiler
	instructions bytecode.Instructions // Instructions generated by compiler
	stack        []object.Object       // Stack for operands
	stackPointer int                   // stack[stackPointer-1] is top of stack
	globals      []object.Object       // Globals
}

func BuildVM(bytecode *compiler.Bytecode) *VM {
	return &VM{
		instructions: bytecode.Instructions,
		constants:    bytecode.Constants,
		stack:        make([]object.Object, stackCapacity),
		stackPointer: 0,
		globals:      make([]object.Object, GlobalCapacity),
	}
}

func BuildStatefulVM(bytecode *compiler.Bytecode, g []object.Object) *VM {
	vm := BuildVM(bytecode)
	vm.globals = g
	return vm
}

// Fetch-decode-execute cycle (instruction cycle)
func (vm *VM) Run() error {
	for i := 0; i < len(vm.instructions); i++ {
		// Fetch
		op := bytecode.Opcode(vm.instructions[i])

		// Decode
		switch op {
		case bytecode.OpGetGlobal:
			// Execute
			globalIndex := bytecode.ReadUint16(vm.instructions[i+1:])
			i += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}
		case bytecode.OpSetGlobal:
			// Execute
			globalIndex := bytecode.ReadUint16(vm.instructions[i+1:])
			i += 2
			vm.globals[globalIndex] = vm.pop()
		case bytecode.OpNull:
			// Execute
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case bytecode.OpJumpNotTruthy:
			// Execute
			position := int(bytecode.ReadUint16(vm.instructions[i+1:]))
			// Skip over operand
			i += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				i = position - 1
			}
		case bytecode.OpJump:
			//Execute
			position := int(bytecode.ReadUint16(vm.instructions[i+1:]))
			// -1 because loop increments i
			i = position - 1
		case bytecode.OpConstant:
			// Execute
			constIndex := bytecode.ReadUint16(vm.instructions[i+1:])
			// Skip over operand
			i += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}
		case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv:
			//Execute
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}
		case bytecode.OpPop:
			//Execute
			vm.pop()
		case bytecode.OpTrue:
			// Execute
			err := vm.push(True)
			if err != nil {
				return err
			}
		case bytecode.OpFalse:
			// Execute
			err := vm.push(False)
			if err != nil {
				return err
			}
		case bytecode.OpEqual, bytecode.OpNotEqual, bytecode.OpGreater:
			// Execute
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}
		case bytecode.OpBang:
			// Execute
			err := vm.executeBang()
			if err != nil {
				return err
			}
		case bytecode.OpMinus:
			// Execute
			err := vm.executeMinus()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper method for conditionals
func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

// Helper method to execute -
func (vm *VM) executeMinus() error {
	value := vm.pop()

	if value.Type() != object.INTEGER_OBJECT {
		return fmt.Errorf("Unsupported type: %s", value.Type())
	}

	return vm.push(&object.Integer{Value: -value.(*object.Integer).Value})
}

// Helper method to execute !
func (vm *VM) executeBang() error {
	value := vm.pop()

	switch value {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

// Helper method to execute !=, >, ==
func (vm *VM) executeComparison(op bytecode.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJECT || right.Type() == object.INTEGER_OBJECT {
		return vm.executeIntegerComparison(left, op, right)
	}

	switch op {
	case bytecode.OpEqual:
		return vm.push(toBooleanObject(right == left))
	case bytecode.OpNotEqual:
		return vm.push(toBooleanObject(right != left))
	default:
		return fmt.Errorf("Unknown operator: %s %d %s", left.Type(), op, right.Type())
	}
}

// Helper method to execute !=, >, == for integers
func (vm *VM) executeIntegerComparison(
	left object.Object, op bytecode.Opcode, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case bytecode.OpEqual:
		return vm.push(toBooleanObject(leftValue == rightValue))
	case bytecode.OpNotEqual:
		return vm.push(toBooleanObject(leftValue != rightValue))
	case bytecode.OpGreater:
		return vm.push(toBooleanObject(leftValue > rightValue))
	default:
		return fmt.Errorf("Unknown operator: %d", op)
	}
}

// Helper method to convert bool to boolean objects
func toBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	} else {
		return False
	}
}

// Helper method to execute +,-,*,/
func (vm *VM) executeBinaryOperation(op bytecode.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJECT && right.Type() == object.INTEGER_OBJECT {
		leftValue := left.(*object.Integer).Value
		rightValue := right.(*object.Integer).Value

		var result int64

		switch op {
		case bytecode.OpAdd:
			result = leftValue + rightValue
		case bytecode.OpSub:
			result = leftValue - rightValue
		case bytecode.OpMul:
			result = leftValue * rightValue
		case bytecode.OpDiv:
			result = leftValue / rightValue
		default:
			return fmt.Errorf("Unsupported operator for integer: %s", op)
		}

		return vm.push(&object.Integer{Value: result})
	} else if left.Type() == object.STRING_OBJECT && right.Type() == object.STRING_OBJECT {
		if op != bytecode.OpAdd {
			return fmt.Errorf("Unsupported operator for string: %s", op)
		}

		leftValue := left.(*object.String).Value
		rightValue := right.(*object.String).Value

		return vm.push(&object.String{Value: leftValue + rightValue})
	} else {
		return fmt.Errorf("Unsupported types for binary operation: %s %s", left.Type(), right.Type())
	}
}

// Get last popped element (for debugging)
func (vm *VM) LastPopped() object.Object {
	return vm.stack[vm.stackPointer]
}

// Push to stack
func (vm *VM) push(o object.Object) error {
	if vm.stackPointer >= stackCapacity {
		return fmt.Errorf("Stack overflow")
	}

	vm.stack[vm.stackPointer] = o
	vm.stackPointer++
	return nil
}

// Pop from stack
func (vm *VM) pop() object.Object {
	o := vm.stack[vm.stackPointer-1]
	vm.stackPointer--
	return o
}
