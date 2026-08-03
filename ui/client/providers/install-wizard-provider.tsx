import { useMutation, useQuery } from "@tanstack/react-query";
import {
  createContext,
  useEffect,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useNavigate, useSearchParams } from "react-router";
import {
  createCustomerInstallWizardInstall,
  getCustomerInstallWizardState,
  type TCustomerWizardCreatePayload,
  type TCustomerWizardGroup,
  type TCustomerWizardState,
  type TCustomerWizardStep,
} from "@/lib/api/customer/install-wizard";
import { extractSubdomain } from "@/utils/subdomain-utils";
import {
  aggregateGroupStatus,
  type TInstallWizardFormValues,
} from "@/components/customer/install-wizard/wizard-utils";

type TInstallWizardStepState = {
  status: string;
  accessible: boolean;
};

type TInstallWizardStepStateMap = Record<
  TCustomerWizardStep,
  TInstallWizardStepState
>;

const WIZARD_STEPS: TCustomerWizardStep[] = [
  "inputs",
  "stack",
  "sandbox",
  "components",
];

type TInstallWizardContextValue = {
  appId: string;
  currentStep: TCustomerWizardStep;
  installId?: string;
  workflowId?: string;
  stepStates: TInstallWizardStepStateMap;
  shouldPoll: boolean;
  wizardState: TCustomerWizardState | null;
  isLoading: boolean;
  loadError: string | null;
  createError: string | null;
  actionError: string | null;
  isCreating: boolean;
  actionPending: boolean;
  confirmInstall: (values: TInstallWizardFormValues) => void;
  moveToNextStep: () => void;
  goToStep: (targetStep: TCustomerWizardStep) => void;
  approve: (group: TCustomerWizardGroup) => Promise<void>;
  approveAll: () => Promise<void>;
  retry: (group: TCustomerWizardGroup) => Promise<void>;
};

export const InstallWizardContext =
  createContext<TInstallWizardContextValue | null>(null);

function normalizeWizardStep(step: string | null): TCustomerWizardStep {
  switch (step) {
    case "stack":
    case "sandbox":
    case "components":
      return step;
    default:
      return "inputs";
  }
}

export function createWizardStepStates(
  currentStep: TCustomerWizardStep,
  installId?: string,
): TInstallWizardStepStateMap {
  const hasExistingInstall = Boolean(installId);
  const effectiveStep =
    currentStep === "inputs" && hasExistingInstall ? "stack" : currentStep;
  const currentIndex = WIZARD_STEPS.indexOf(effectiveStep);
  const states = {} as TInstallWizardStepStateMap;

  for (const [index, step] of WIZARD_STEPS.entries()) {
    if (step === "inputs") {
      states[step] = {
        status:
          currentStep === "inputs" && !hasExistingInstall
            ? "in-progress"
            : "completed",
        accessible: true,
      };
      continue;
    }

    if (index < currentIndex) {
      states[step] = { status: "completed", accessible: true };
      continue;
    }

    if (index === currentIndex) {
      states[step] = { status: "in-progress", accessible: true };
      continue;
    }

    states[step] = { status: "upcoming", accessible: false };
  }

  return states;
}

function stepStatusWeight(status: string): number {
  switch (status) {
    case "upcoming":
      return 0;
    case "in-progress":
      return 1;
    case "approval-awaiting":
      return 2;
    case "completed":
    case "success":
      return 3;
    case "error":
      return 4;
    default:
      return 0;
  }
}

export function mergeWizardStepStates(
  previous: TInstallWizardStepStateMap,
  next: TInstallWizardStepStateMap,
): TInstallWizardStepStateMap {
  let changed = false;
  const merged = {} as TInstallWizardStepStateMap;

  for (const step of WIZARD_STEPS) {
    const previousState = previous[step];
    const nextState = next[step];

    if (!previousState) {
      merged[step] = nextState;
      changed = true;
      continue;
    }

    const usePrevious =
      stepStatusWeight(previousState.status) >
      stepStatusWeight(nextState.status);

    const status = usePrevious ? previousState.status : nextState.status;
    const accessible = previousState.accessible || nextState.accessible;

    merged[step] = { status, accessible };

    if (
      status !== previousState.status ||
      accessible !== previousState.accessible
    ) {
      changed = true;
    }
  }

  return changed ? merged : previous;
}

function shouldPollWizardStep(
  currentStep: TCustomerWizardStep,
  workflow?: TCustomerWizardState["workflow"],
): boolean {
  if (currentStep === "inputs") {
    return false;
  }

  if (!workflow) {
    return true;
  }

  if (workflow.is_step_complete || workflow.is_step_error) {
    return false;
  }

  if (workflow.groups.length === 0) {
    return true;
  }

  const status = aggregateGroupStatus(workflow.groups);
  return status === "in-progress" || status === "approval-awaiting";
}

function isStepNavigable(
  stepState: TInstallWizardStepState | undefined,
): boolean {
  if (!stepState) {
    return false;
  }

  switch (stepState.status) {
    case "completed":
    case "success":
    case "in-progress":
    case "approval-awaiting":
    case "error":
      return true;
    default:
      return false;
  }
}

function isStepTerminalSuccess(
  stepState: TInstallWizardStepState | undefined,
): boolean {
  return stepState?.status === "completed" || stepState?.status === "success";
}

function buildPortalActionURL(pathname: string) {
  const search = new URLSearchParams();
  const configuredBaseDomain =
    (import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN as string | undefined) ?? "";
  const fallbackSubdomain =
    (import.meta.env.VITE_CUSTOMER_SUBDOMAIN as string | undefined) ?? "";

  const inferredBaseDomain = (() => {
    if (configuredBaseDomain) {
      return configuredBaseDomain;
    }

    const [hostname, port] = window.location.host.split(":");
    if (!hostname) {
      return window.location.host;
    }

    const labels = hostname.split(".");
    if (labels.length <= 1) {
      return window.location.host;
    }

    const host = labels.slice(1).join(".");
    return port ? `${host}:${port}` : host;
  })();

  const subdomain =
    extractSubdomain(window.location.host, inferredBaseDomain) ||
    fallbackSubdomain;
  if (subdomain) {
    search.set("subdomain", subdomain);
  }

  return `${pathname}${search.toString() ? `?${search.toString()}` : ""}`;
}

async function performAction(url: string, body?: URLSearchParams) {
  const response = await fetch(`/bff${url}`, {
    method: "POST",
    credentials: "include",
    headers: body
      ? {
          "Content-Type": "application/x-www-form-urlencoded",
          Accept: "text/html,application/json",
        }
      : { Accept: "text/html,application/json" },
    body: body?.toString(),
  });
  if (!response.ok) {
    throw new Error(`Workflow action failed with status ${response.status}`);
  }
}

export function InstallWizardProvider({
  appId,
  children,
}: {
  appId: string;
  children: ReactNode;
}) {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const step = normalizeWizardStep(searchParams.get("step"));
  const installId = searchParams.get("install_id") ?? undefined;
  const workflowId = searchParams.get("workflow_id") ?? undefined;
  const [createError, setCreateError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [forcePollUntil, setForcePollUntil] = useState<number>(0);
  const computedStepStates = useMemo(
    () => createWizardStepStates(step, installId),
    [installId, step],
  );
  const [stepStates, setStepStates] =
    useState<TInstallWizardStepStateMap>(computedStepStates);
  const stepStateSessionRef = useRef(`${appId}:${installId ?? "new"}`);

  useEffect(() => {
    const sessionKey = `${appId}:${installId ?? "new"}`;

    if (stepStateSessionRef.current !== sessionKey) {
      stepStateSessionRef.current = sessionKey;
      setStepStates(computedStepStates);
      return;
    }

    setStepStates((previous) =>
      mergeWizardStepStates(previous, computedStepStates),
    );
  }, [appId, computedStepStates, installId]);

  // Component deploys can run in a newer workflow than the one used for
  // stack/sandbox. Querying components with the pinned URL workflow can leave
  // the wizard waiting on an old workflow that has no component groups.
  const requestedWorkflowId = step === "components" ? undefined : workflowId;

  const query = useQuery({
    queryKey: [
      "customer",
      "wizard",
      appId,
      step,
      installId,
      requestedWorkflowId,
    ],
    queryFn: () =>
      getCustomerInstallWizardState({
        appId,
        step,
        installId,
        workflowId: requestedWorkflowId,
      }),
    staleTime: 1_000,
    refetchOnWindowFocus: false,
    refetchInterval: (queryState) => {
      const shouldForcePoll = Date.now() < forcePollUntil;
      const shouldPollFromState = shouldPollWizardStep(
        step,
        queryState.state.data?.workflow,
      );
      return shouldPollFromState || shouldForcePoll ? 3_000 : false;
    },
  });

  const createMutation = useMutation({
    mutationFn: (payload: TCustomerWizardCreatePayload) =>
      createCustomerInstallWizardInstall(appId, payload),
    onSuccess: (result) => {
      setCreateError(null);
      const nextURL = new URL(result.next_url, window.location.origin);
      navigate(`${nextURL.pathname}${nextURL.search}`);
    },
    onError: (error) => {
      setCreateError(
        error instanceof Error
          ? error.message
          : "Unable to create this install right now.",
      );
    },
  });

  const actionMutation = useMutation({
    mutationFn: async ({
      type,
      group,
    }: {
      type: "approve" | "approve-all" | "retry";
      group?: TCustomerWizardGroup;
    }) => {
      const activeWorkflowId = query.data?.workflow_id ?? workflowId;

      if (!installId || !activeWorkflowId) {
        throw new Error("Workflow context is missing.");
      }

      if (type === "approve") {
        if (!group?.approval) {
          throw new Error("Approval data is missing.");
        }
        const form = new URLSearchParams({
          step_id: group.approval.step_id,
          approval_id: group.approval.approval_id,
        });
        await performAction(
          buildPortalActionURL(
            `/portal-api/installs/${installId}/workflows/${activeWorkflowId}/approve`,
          ),
          form,
        );
        return;
      }

      if (type === "approve-all") {
        await performAction(
          buildPortalActionURL(
            `/portal-api/installs/${installId}/workflows/${activeWorkflowId}/approve-all`,
          ),
        );
        return;
      }

      if (!group?.retry_step_id) {
        throw new Error("Retry step is missing.");
      }
      await performAction(
        `/installs/${installId}/workflows/${activeWorkflowId}/step/${group.retry_step_id}/retry`,
      );
    },
    onSuccess: async () => {
      // Approve/retry actions can take a short time to propagate through workflow
      // step groups, so keep polling briefly to surface state changes in the UI.
      setForcePollUntil(Date.now() + 30_000);
      await query.refetch();
    },
    onError: (error) => {
      setActionError(
        error instanceof Error
          ? error.message
          : "Unable to perform that workflow action.",
      );
    },
  });

  const confirmInstall = (values: TInstallWizardFormValues) => {
    if (!query.data?.form) {
      setCreateError("Install form is not ready.");
      return;
    }

    setCreateError(null);
    createMutation.mutate({
      name: values.installName.trim(),
      region: query.data.form.platform === "aws" ? values.region : undefined,
      location:
        query.data.form.platform === "azure" ? values.location : undefined,
      inputs: values.inputs,
    });
  };

  const moveToNextStep = () => {
    if (!query.data?.workflow?.next_step || !installId) {
      return;
    }

    const activeWorkflowId = query.data?.workflow_id ?? workflowId;

    const next = new URLSearchParams(searchParams);
    next.set("step", query.data.workflow.next_step);
    next.set("install_id", installId);
    if (activeWorkflowId) {
      next.set("workflow_id", activeWorkflowId);
    } else {
      next.delete("workflow_id");
    }
    setSearchParams(next, { replace: false });
  };

  const goToStep = (targetStep: TCustomerWizardStep) => {
    const targetState = stepStates[targetStep];
    if (!isStepNavigable(targetState) || targetStep === step) {
      return;
    }

    if (targetStep !== "inputs" && !installId) {
      return;
    }

    const next = new URLSearchParams(searchParams);
    next.set("step", targetStep);

    if (installId) {
      next.set("install_id", installId);
    } else {
      next.delete("install_id");
    }

    const activeWorkflowId = query.data?.workflow_id ?? workflowId;
    if (activeWorkflowId) {
      next.set("workflow_id", activeWorkflowId);
    } else {
      next.delete("workflow_id");
    }

    setSearchParams(next, { replace: false });
  };

  const value = useMemo<TInstallWizardContextValue>(
    () => ({
      appId,
      currentStep: step,
      installId,
      workflowId,
      stepStates,
      shouldPoll: shouldPollWizardStep(step, query.data?.workflow),
      wizardState: query.data ?? null,
      isLoading: query.isLoading,
      loadError:
        query.error instanceof Error
          ? query.error.message
          : query.error
            ? "Unable to load the install wizard."
            : null,
      createError,
      actionError,
      isCreating: createMutation.isPending,
      actionPending: actionMutation.isPending,
      confirmInstall,
      moveToNextStep,
      goToStep,
      approve: async (group) => {
        if (isStepTerminalSuccess(stepStates[step])) {
          return;
        }
        setActionError(null);
        await actionMutation.mutateAsync({ type: "approve", group });
      },
      approveAll: async () => {
        if (isStepTerminalSuccess(stepStates[step])) {
          return;
        }
        setActionError(null);
        await actionMutation.mutateAsync({ type: "approve-all" });
      },
      retry: async (group) => {
        if (isStepTerminalSuccess(stepStates[step])) {
          return;
        }
        setActionError(null);
        await actionMutation.mutateAsync({ type: "retry", group });
      },
    }),
    [
      actionError,
      actionMutation,
      appId,
      createError,
      createMutation.isPending,
      installId,
      query.data,
      query.error,
      query.isLoading,
      searchParams,
      step,
      stepStates,
      workflowId,
    ],
  );

  return (
    <InstallWizardContext.Provider value={value}>
      {children}
    </InstallWizardContext.Provider>
  );
}

export function useInstallWizard() {
  const context = useContext(InstallWizardContext);
  if (!context) {
    throw new Error(
      "useInstallWizard must be used within InstallWizardProvider",
    );
  }
  return context;
}
