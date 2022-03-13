package main

var Global *int

var Func2Ref func()

func NewFunc1() {
	println("NewFunc1: ", *Global)
	*Global += 15
	Func2Ref()
}

func HPBeforeLoad() error {
	println("HPBeforeLoad called")
	return nil
}

func HPAfterLoad() error {
	println("HPAfterLoad called")
	return nil
}

func HPBeforeUnload() error {
	println("HPBeforeUnload called")
	return nil
}

func HPAfterUnload() error {
	println("HPAfterUnload called")
	return nil
}
