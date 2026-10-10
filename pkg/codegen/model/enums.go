package model

type TemplateType int

const (
	TemplateTypeSingle  TemplateType = 1
	TemplateTypeTree    TemplateType = 2
	TemplateTypeMainSub TemplateType = 3
)

func (t TemplateType) Valid() bool { return t >= TemplateTypeSingle && t <= TemplateTypeMainSub }

type FrontType int

const FrontTypeVben5Antd FrontType = 1

func (f FrontType) Valid() bool { return f == FrontTypeVben5Antd }

type HTMLType string

const (
	HTMLInput       HTMLType = "Input"
	HTMLTextarea    HTMLType = "Textarea"
	HTMLInputNumber HTMLType = "InputNumber"
	HTMLSelect      HTMLType = "Select"
	HTMLSwitch      HTMLType = "Switch"
	HTMLDatePicker  HTMLType = "DatePicker"
)
