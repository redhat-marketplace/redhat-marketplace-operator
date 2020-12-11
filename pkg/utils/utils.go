package utils


func Must(in func() (interface{}, error)) interface{} {
	i, err := in()

	if err != nil {
		panic(err)
	}

	return i
}
