import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/utils/classnames";

type TAlertTheme = "warning" | "success" | "info" | "alert";

export interface IMessage extends Omit<
  HTMLAttributes<HTMLDivElement>,
  "title"
> {
  title?: ReactNode;
  theme?: TAlertTheme;
}

const THEME_STYLES: Record<TAlertTheme, string> = {
  warning:
    "border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300",
  success:
    "border-green-200 dark:border-green-800 bg-green-50 dark:bg-[#0B1A13] text-green-800 dark:text-green-300",
  info: "border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-[#0F172A] text-blue-800 dark:text-blue-300",
  alert:
    "border-orange-200 dark:border-orange-800 bg-orange-50 dark:bg-[#2D1E10] text-orange-800 dark:text-orange-300",
};

export const Message = ({
  children,
  className,
  title,
  theme = "warning",
  ...props
}: IMessage) => {
  return (
    <div
      className={cn(
        "rounded-lg border px-4 py-3 text-sm",
        THEME_STYLES[theme],
        className,
      )}
      {...props}
    >
      {title ? <strong>{title} </strong> : null}
      {children}
    </div>
  );
};

export type IAlertMessage = IMessage;

export const AlertMessage = (props: IMessage) => <Message {...props} />;

export interface IWarningMessage extends Omit<IMessage, "theme"> {}

export const WarningMessage = (props: IWarningMessage) => (
  <Message theme="warning" {...props} />
);
