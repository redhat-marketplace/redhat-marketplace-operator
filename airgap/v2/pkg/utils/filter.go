package main

import (
	"fmt"
	"log"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/interpreter"

	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func main() {
	//paramA := decls.NewTypeParamType("A")
	//typeParamAList := []string{"A"}
	optimizeArith := func(i interpreter.Interpretable) (interpreter.Interpretable, error) {
		// Only optimize the instruction if it is a call.
		call, ok := i.(interpreter.InterpretableCall)
		if !ok {
			return i, nil
		}
		// Only optimize the math functions when they have constant arguments.
		switch call.Function() {
		case operators.Equals:
			// These are all binary operators so they should have to arguments
			args := call.Args()
			ans := types.String(
				fmt.Sprintf("%s == %s",
					args[0].Eval(interpreter.EmptyActivation()),
					args[1].Eval(interpreter.EmptyActivation())))
			return interpreter.NewConstValue(call.ID(), ans), nil
		default:
			return i, nil
		}
	}

	env, err := cel.NewCustomEnv(d)
	if err != nil {
		log.Fatalf("environment creation error: %v\n", err)
	}

	ast, iss := env.Compile(`source == "foo"`)
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		log.Fatalf("Program creation error: %v\n", err)
	}

	out, _, err := prg.Eval(map[string]interface{}{})

	if err != nil {
		log.Fatalf("Program eval error: %v\n", err.Error())
	}

	fmt.Printf("%s", out.Value())

}

func main2() {
	d := cel.Declarations(
		decls.NewConst("size", decls.String, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "size"}}),
		decls.NewConst("source", decls.String, &exprpb.Constant{ConstantKind: &exprpb.Constant_StringValue{StringValue: "source"}}),
		decls.NewFunction("-_", decls.NewOverload("-_override", []*exprpb.Type{decls.String}, decls.String)),
	)

	inverseFunc := &functions.Overload{
		Operator: "-_override",
		Unary: func(val ref.Val) ref.Val {
			return types.String(fmt.Sprintf("%s desc", val))
		},
	}

	env, err := cel.NewEnv(d)
	if err != nil {
		log.Fatalf("environment creation error: %v\n", err)
	}

	ast, iss := env.Compile(`[size, -source]`)
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	prg, err := env.Program(ast, cel.Functions(inverseFunc))
	if err != nil {
		log.Fatalf("Program creation error: %v\n", err)
	}

	out, _, err := prg.Eval(map[string]interface{}{})

	v, _ := out.ConvertToNative(reflect.SliceOf(reflect.TypeOf("")))
	fmt.Println(v.([]string)[0])
	fmt.Println(v.([]string)[1])
	fmt.Printf("%+v", v)

	// d := cel.Declarations(
	// 	decls.NewVar("key", decls.String),
	// 	decls.NewVar("value", decls.String),
	// 	decls.NewFunction("",
	// 		decls.NewOverload("shake_hands_string_string",
	// 			[]*exprpb.Type{decls.String, decls.String},
	// 			decls.String)))
	// env, err := cel.NewEnv(d)
	// if err != nil {
	// 	log.Fatalf("environment creation error: %v\n", err)
	// }
	// // Check iss for error in both Parse and Check.
	// ast, iss := env.Compile(`shake_hands(i,you)`)
	// if iss.Err() != nil {
	// 	log.Fatalln(iss.Err())
	// }
	// shakeFunc := &functions.Overload{
	// 	Operator: "shake_hands_string_string",
	// 	Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
	// 		return types.String(
	// 			fmt.Sprintf("%s and %s are shaking hands.\n", lhs, rhs))
	// 	}}
	// prg, err := env.Program(ast, cel.Functions(shakeFunc))
	// if err != nil {
	// 	log.Fatalf("Program creation error: %v\n", err)
	// }

	// out, _, err := prg.Eval(map[string]interface{}{
	// 	"i":   "CEL",
	// 	"you": "world",
	// })
	// if err != nil {
	// 	log.Fatalf("Evaluation error: %v\n", err)
	// }

	// fmt.Println(out)

}
