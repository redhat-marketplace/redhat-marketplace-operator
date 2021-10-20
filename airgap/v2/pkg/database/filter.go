package database

import (
	"errors"
	"fmt"
	"log"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
	"github.com/google/cel-go/interpreter/functions"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

const (
	equalsQuery   = "equals_query"
	notEqualQuery = "not_equals_query"

	greaterThanQuery      = ">_query"
	lessThanQuery         = "<_query"
	greaterThanEqualQuery = ">=_query"
	lessThanEqualQuery    = "<=_query"

	andQuery = "and_query"
	orQuery  = "or_query"
)

var (
	d = cel.Declarations(
		decls.NewConst("size", decls.String, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "size"}}),
		decls.NewConst("source", decls.String, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "source"}}),
		decls.NewConst("sourceType", decls.String, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "sourceType"}}),
		decls.NewConst("createdAt", decls.Int, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "createdAt"}}),
		decls.NewConst("deletedAt", decls.Int, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "deletedAt"}}),

		decls.NewFunction(operators.Equals,
			decls.NewOverload(equalsQuery,
				[]*exprpb.Type{decls.Any, decls.Any}, decls.String),
		),
		decls.NewFunction(operators.NotEquals,
			decls.NewOverload(notEqualQuery,
				[]*exprpb.Type{decls.Any, decls.Any}, decls.String),
		),

		decls.NewFunction(operators.LogicalAnd,
			decls.NewOverload(andQuery, []*exprpb.Type{decls.String, decls.String}, decls.String),
			decls.NewInstanceOverload(andQuery, []*exprpb.Type{decls.String, decls.String}, decls.String),
		),
		decls.NewFunction(operators.LogicalOr,
			decls.NewOverload(orQuery, []*exprpb.Type{decls.String, decls.String}, decls.String),
		),

		decls.NewFunction(operators.Greater, decls.NewOverload(greaterThanQuery,
			[]*exprpb.Type{decls.Int, decls.Int}, decls.String)),
		decls.NewFunction(operators.GreaterEquals, decls.NewOverload(greaterThanEqualQuery,
			[]*exprpb.Type{decls.Int, decls.Int}, decls.String)),

		decls.NewFunction(operators.Less, decls.NewOverload(lessThanQuery,
			[]*exprpb.Type{decls.Int, decls.Int}, decls.String)),
		decls.NewFunction(operators.LessEquals, decls.NewOverload(lessThanEqualQuery,
			[]*exprpb.Type{decls.Int, decls.Int}, decls.String)),
	)

	greaterFunc = &functions.Overload{
		Operator:     greaterThanQuery,
		OperandTrait: 0,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s > %d", lhs, rhs.Value()))
		},
	}
	greaterEqualFunc = &functions.Overload{
		Operator:     greaterThanEqualQuery,
		OperandTrait: 0,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s >= %d", lhs, rhs.Value()))
		},
	}
	lessFunc = &functions.Overload{
		Operator:     lessThanQuery,
		OperandTrait: 0,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s < %d", lhs, rhs.Value()))
		},
	}
	lessEqualFunc = &functions.Overload{
		Operator:     lessThanEqualQuery,
		OperandTrait: 0,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s <= %d", lhs, rhs.Value()))
		},
	}

	equalsFunc = &functions.Overload{
		Operator: equalsQuery,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s == %s", lhs, rhs))
		},
	}

	notEqualsFunc = &functions.Overload{
		Operator: notEqualQuery,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s <> %s", lhs, rhs))
		},
	}

	andFunc = &functions.Overload{
		Operator: andQuery,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s AND %s", lhs, rhs))
		},
	}
	orFunc = &functions.Overload{
		Operator: orQuery,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s OR %s", lhs, rhs))
		},
	}

	programOpts = []cel.ProgramOption{
		cel.Functions(greaterEqualFunc, lessEqualFunc, lessFunc, greaterFunc, equalsFunc, notEqualsFunc, andFunc, orFunc),
		cel.CustomDecorator(filterInterpreter),
		cel.EvalOptions(cel.OptPartialEval),
	}

	env *cel.Env

	binds = map[string]interface{}{}
)

func filterInterpreter(i interpreter.Interpretable) (interpreter.Interpretable, error) {
	fmt.Println(i.ID())

	v := i.Eval(interpreter.EmptyActivation())
	fmt.Printf("%+v\n", v.Value())

	// Only optimize the instruction if it is a call.
	call, ok := i.(interpreter.InterpretableCall)
	if !ok {
		return i, nil
	}

	fmt.Println(call.Function())

	switch call.Function() {
	case operators.Equals,
		operators.NotEquals,
		operators.LogicalAnd,
		operators.LogicalOr:
		// These are all binary operators so they should have to arguments
		args := call.Args()

		funcSymbol := ""
		switch call.Function() {
		case operators.Equals:
			funcSymbol = "=="
		case operators.NotEquals:
			funcSymbol = "<>"
		case operators.LogicalAnd:
			funcSymbol = "AND"
		case operators.LogicalOr:
			funcSymbol = "OR"
		}

		v1 := args[1].Eval(interpreter.EmptyActivation())
		val1 := fmt.Sprintf("%s", v1.Value())
		if v1.Type() == types.StringType {
			val1 = fmt.Sprintf(`"%s"`, v1.Value())
		}

		ans := types.String(
			fmt.Sprintf("%s %s %s",
				args[0].Eval(interpreter.EmptyActivation()),
				funcSymbol,
				val1))
		return interpreter.NewConstValue(call.ID(), ans), nil
	default:
		return i, nil
	}
}

func init() {
	var err error
	env, err = cel.NewCustomEnv(d, cel.ClearMacros())
	if err != nil {
		log.Fatalf("environment creation error: %v\n", err)
	}
}

func ParseFilter(filter string) (string, error) {
	ast, iss := env.Compile(filter)

	if iss.Err() != nil {
		return "", iss.Err()
	}

	prg, err := env.Program(ast, programOpts...)

	if err != nil {
		fmt.Println("program error", err)
		return "", err
	}

	out, _, err := prg.Eval(binds)

	if err != nil {
		fmt.Println("eval error", err.Error())
		return "", err
	}

	fmt.Printf("%v\n", out.Value())
	filterQuery, ok := out.Value().(string)
	if !ok {
		return "", errors.New("result was not a string")
	}

	return filterQuery, err
}
