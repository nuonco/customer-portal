import {
  type FocusEvent,
  type FormEvent,
  type InputHTMLAttributes,
  type ReactNode,
  forwardRef,
  useCallback,
  useState,
} from "react";
import { Label, type ILabel } from "@/components/common/form/Label";
import { Text, type IText } from "@/components/common/Text";
import { cn } from "@/utils/classnames";

export interface IInput extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "size"
> {
  labelProps?: Omit<ILabel, "children"> & {
    labelText: ReactNode;
    labelTextProps?: Omit<IText, "children">;
  };
  helperText?: string;
  helperTextProps?: Omit<IText, "children">;
  error?: boolean;
  errorMessage?: string;
  errorMessageProps?: Omit<IText, "children">;
  size?: "sm" | "md" | "lg";
}

export const Input = forwardRef<HTMLInputElement, IInput>(
  (
    {
      className,
      labelProps,
      helperText,
      helperTextProps = { variant: "subtext" },
      error,
      errorMessage,
      errorMessageProps = { variant: "subtext", theme: "error" },
      size = "md",
      disabled,
      required,
      onBlur,
      onInvalid,
      ...props
    },
    ref,
  ) => {
    // Whether the user has blurred the field or tried to submit the form. Until
    // then we stay quiet, so a pristine form is not covered in errors.
    const [hasInteracted, setHasInteracted] = useState(false);

    // Validity is DERIVED during render from the controlled value rather than
    // mirrored into state and re-synced from native DOM events. The previous
    // implementation only recovered when a native `input` event fired, so any
    // value change that did not produce one — a React-driven update, autofill,
    // a programmatic set — left "Please fill out this field" stranded over a
    // filled field.
    //
    // Every call site is controlled; the DOM read is a fallback for uncontrolled
    // use, where there is no prop to derive from.
    const isEmpty =
      props.value !== undefined
        ? props.value === "" || props.value === null
        : false;
    const failsRequired = Boolean(required) && isEmpty;
    const showRequiredMessage = hasInteracted && failsRequired;

    // `error` is parent-driven; other native constraints (pattern, type, etc.)
    // are handled by the `user-invalid:` classes below, which are native CSS and
    // therefore cannot go stale.
    const isInvalid = Boolean(error) || showRequiredMessage;

    const handleBlur = useCallback(
      (e: FocusEvent<HTMLInputElement>) => {
        setHasInteracted(true);
        onBlur?.(e);
      },
      [onBlur],
    );

    const handleInvalid = useCallback(
      (e: FormEvent<HTMLInputElement>) => {
        // Suppress the native validation bubble; we render the message inline.
        e.preventDefault();
        setHasInteracted(true);
        onInvalid?.(e);
      },
      [onInvalid],
    );

    const setRefs = useCallback(
      (node: HTMLInputElement | null) => {
        if (typeof ref === "function") {
          ref(node);
        } else if (ref) {
          ref.current = node;
        }
      },
      [ref],
    );

    const sizeClasses = {
      sm: "px-2 py-1 text-sm h-8",
      md: "px-3 py-2 text-sm h-9",
      lg: "px-4 py-3 text-base h-12",
    };

    const baseClasses = cn(
      "w-full rounded-md border transition-colors duration-200",
      "bg-white dark:bg-dark-grey-900",
      "shadow-[0px_1px_2px_0px_rgba(0,0,0,0.08)]",
      "placeholder:text-cool-grey-500 dark:placeholder:text-cool-grey-600",
      "font-sans",

      "focus:outline-none focus:ring-2 focus:ring-primary-500 focus:!border-primary-500",

      "user-invalid:!border-red-500 user-invalid:dark:!border-red-400",
      "user-invalid:focus:!border-red-500 user-invalid:focus:!ring-red-500",

      sizeClasses[size],

      {
        "border-cool-grey-500/24 dark:border-cool-grey-500/24":
          !error && !disabled && !isInvalid,
        "text-cool-grey-900 dark:text-cool-grey-100": !disabled,

        "!border-red-500 dark:!border-red-400": error || isInvalid,
        "focus:!ring-red-500 focus:!border-red-500": error || isInvalid,

        "!border-cool-grey-300 dark:!border-dark-grey-600": disabled,
        "!bg-cool-grey-100 dark:!bg-dark-grey-700": disabled,
        "text-cool-grey-400 dark:text-cool-grey-500": disabled,
        "cursor-not-allowed": disabled,
        "!shadow-none": disabled,
        "focus:!ring-transparent focus:!border-cool-grey-300 dark:focus:!border-dark-grey-600":
          disabled,
      },
      className,
    );

    const input = (
      <input
        ref={setRefs}
        className={baseClasses}
        disabled={disabled}
        required={required}
        aria-invalid={isInvalid}
        aria-describedby={
          helperText || errorMessage || showRequiredMessage
            ? `${props.id}-description`
            : undefined
        }
        onBlur={handleBlur}
        onInvalid={handleInvalid}
        {...props}
      />
    );

    const renderDescription = () => {
      if (error && errorMessage) {
        return (
          <Text
            id={`${props.id}-description`}
            className={cn("block", errorMessageProps?.className)}
            {...errorMessageProps}
          >
            {errorMessage}
          </Text>
        );
      }

      if (showRequiredMessage) {
        return (
          <Text
            id={`${props.id}-description`}
            variant="subtext"
            theme="error"
            className="mt-1"
          >
            Please fill out this field
          </Text>
        );
      }

      if (helperText) {
        return (
          <Text
            id={`${props.id}-description`}
            className={cn("block", helperTextProps?.className)}
            theme="neutral"
            {...helperTextProps}
          >
            {helperText}
          </Text>
        );
      }

      return null;
    };

    if (labelProps) {
      const { labelText, labelTextProps, ...restLabelProps } = labelProps;
      return (
        <div className="space-y-1">
          <Label
            className={cn("block", labelProps.className)}
            htmlFor={props.id}
            {...restLabelProps}
          >
            <Text
              className={cn("font-medium", labelTextProps?.className)}
              variant="body"
              {...labelTextProps}
            >
              <>{labelText}</>
            </Text>
          </Label>
          {input}
          {renderDescription()}
        </div>
      );
    }

    return (
      <div className="space-y-1">
        {input}
        {renderDescription()}
      </div>
    );
  },
);

Input.displayName = "Input";
