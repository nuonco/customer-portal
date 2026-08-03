import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import "../client/styles.css";

type ProviderProps = {
  children: ReactNode;
};

export const Provider = ({ children }: ProviderProps) => (
  <MemoryRouter initialEntries={["/"]}>{children}</MemoryRouter>
);
