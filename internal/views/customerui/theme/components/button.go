package components

type ButtonSize string

const (
	ButtonSizeDefault ButtonSize = ""
	ButtonSizeSm      ButtonSize = "sm"
)

type ButtonVariant string

const (
	ButtonPrimary ButtonVariant = "primary"
	ButtonOutline ButtonVariant = "outline"
	ButtonNeutral ButtonVariant = "neutral"
)

type ButtonProps struct {
	Size     ButtonSize
	Variant  ButtonVariant
	Class    string
	Disabled bool
}

func ButtonClasses(props ButtonProps) string {
	variant := props.Variant
	if variant == "" {
		variant = ButtonPrimary
	}

	classes := "button inline-flex items-center gap-2"

	// Semantic hook class
	classes += " button-" + string(variant)

	// Color class
	switch variant {
	case ButtonPrimary:
		classes += " btn-theme-primary"
	case ButtonOutline:
		classes += " btn-theme-outline"
	case ButtonNeutral:
		classes += " btn-neutral"
	}

	// Size
	switch props.Size {
	case ButtonSizeSm:
		classes += " btn-sm"
	default:
	}

	if props.Disabled {
		classes += " disabled:opacity-40 disabled:cursor-not-allowed"
	}

	if props.Class != "" {
		classes += " " + props.Class
	}

	return classes
}
