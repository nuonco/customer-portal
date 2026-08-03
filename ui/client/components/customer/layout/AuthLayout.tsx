import { Outlet } from "react-router";
import { Button } from "@/components/common/Button";
import { Card } from "@/components/common/Card";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { useCustomerAuth } from "@/hooks/use-customer-auth";
import { Logo } from "@/components/common/Logo";

export const CustomerAuthLayout = () => {
  const { isAuthenticated, isLoading, hasError, retry, startLogin } =
    useCustomerAuth();

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <Text variant="body" theme="neutral">
          Checking your customer session...
        </Text>
      </div>
    );
  }

  if (hasError) {
    return (
      <div className="mx-auto flex min-h-screen w-full max-w-3xl items-center px-4">
        <div className="flex w-full flex-col gap-4">
          <Message theme="warning">
            Unable to verify your customer session right now.
          </Message>
          <div className="flex gap-2">
            <Button onClick={retry} variant="secondary" size="md">
              Retry
            </Button>
            <Button
              onClick={() => void startLogin()}
              variant="primary"
              size="md"
            >
              Sign in
            </Button>
          </div>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen bg-app-bg">
        <div className="flex min-h-screen w-full">
          <div className="flex w-full flex-col justify-center gap-10 bg-app-bg px-8 py-16 md:px-20 lg:w-210">
            <Card className="w-full flex flex-col gap-8 border border-border-strong/25 bg-surface px-8 py-12 shadow-sm md:px-17.5 md:py-16">
              <Logo />
              <div className="flex flex-col gap-6">
                <Text as="h1" variant="h1" weight="stronger">
                  Customer Portal
                </Text>
                <Text variant="h3" theme="neutral">
                  Manage your install experience.
                </Text>
              </div>

              <Button
                onClick={() => void startLogin()}
                variant="primary"
                size="lg"
                className="w-full justify-center"
              >
                Sign in with your organization
              </Button>
            </Card>

            <a
              href="https://docs.nuon.co"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex w-fit items-center gap-1.5 rounded-md border border-transparent px-3 py-1 text-sm font-strong tracking-tight text-primary-600 transition-colors hover:bg-black/5 dark:text-primary-500 dark:hover:bg-white/5"
            >
              Learn more about how Nuon works
              <span aria-hidden="true">↗</span>
            </a>
          </div>

          <div className="relative hidden flex-1 overflow-hidden lg:flex">
            <img
              src="/bff/static/images/oss-hero.png"
              alt=""
              className="absolute inset-0 h-full w-full object-cover"
            />
          </div>
        </div>
      </div>
    );
  }

  return <Outlet />;
};
