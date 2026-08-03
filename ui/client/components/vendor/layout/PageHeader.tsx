import type { ReactNode } from "react";
import { Text } from "@/components/common/Text";

interface IPageHeader {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}

export const PageHeader = ({ title, subtitle, actions }: IPageHeader) => {
  return (
    <div className="mb-10 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
      <div className="flex flex-col gap-1">
        <Text as="h1" variant="h3" weight="stronger">
          {title}
        </Text>
        {subtitle ? (
          <Text as="p" variant="body" theme="neutral">
            {subtitle}
          </Text>
        ) : null}
      </div>

      {actions ? <div>{actions}</div> : null}
    </div>
  );
};
