import { type ReactNode } from "react";

type THeaderAlign = "left" | "center" | "right";

export interface ISimpleTableHeader {
  key: string;
  label: ReactNode;
  align?: THeaderAlign;
  className?: string;
}

interface ISimpleTable {
  headers: ISimpleTableHeader[];
  children: ReactNode;
  bodyClassName?: string;
}

const ALIGNMENT_CLASS: Record<THeaderAlign, string> = {
  left: "text-left",
  center: "text-center",
  right: "text-right",
};

export const SimpleTable = ({
  headers,
  children,
  bodyClassName = "",
}: ISimpleTable) => {
  return (
    <div className="overflow-hidden rounded-lg border border-cool-grey-300 bg-white dark:border-dark-grey-500 dark:bg-dark-grey-900">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-cool-grey-200 dark:divide-dark-grey-700">
          <thead className="bg-cool-grey-50 dark:bg-dark-grey-800">
            <tr>
              {headers.map((header) => (
                <th
                  key={header.key}
                  scope="col"
                  className={`px-4 py-3 text-xs font-medium uppercase tracking-wider text-cool-grey-600 dark:text-cool-grey-400 ${ALIGNMENT_CLASS[header.align ?? "left"]} ${header.className ?? ""}`}
                >
                  {header.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody
            className={`divide-y divide-cool-grey-200 bg-white dark:divide-dark-grey-700 dark:bg-dark-grey-900 ${bodyClassName}`}
          >
            {children}
          </tbody>
        </table>
      </div>
    </div>
  );
};
