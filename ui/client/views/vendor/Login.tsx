import { useCallback } from "react";
import { Button } from "@/components/common/Button";
import { Logo } from "@/components/common/Logo";
import { Card } from "@/components/common/Card";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { useVendorLoginConfig } from "@/hooks/use-vendor-login-config";

export const VendorLoginView = () => {
  const { authUrl, errorMessage, isLoading, retry, title } =
    useVendorLoginConfig();

  const startLogin = useCallback(() => {
    if (!authUrl) {
      return;
    }

    window.location.assign(authUrl);
  }, [authUrl]);

  return (
    <div className="min-h-screen bg-app-bg">
      <div className="flex min-h-screen w-full">
        <div className="flex w-full flex-col justify-center gap-10 bg-app-bg px-8 py-16 md:px-20 lg:w-[840px]">
          <Card className="w-full flex flex-col gap-8 border border-border-strong/25 bg-surface px-8 py-12 shadow-sm md:px-[70px] md:py-16">
            <Logo />
            <div className="flex flex-col gap-6">
              <Text as="h1" variant="h1" weight="stronger">
                {title}
              </Text>
              <Text variant="h3" theme="neutral">
                Manage your customer's install experience.
              </Text>
            </div>

            {errorMessage ? (
              <Message theme="warning">{errorMessage}</Message>
            ) : null}

            {isLoading ? (
              <Text variant="body" theme="neutral">
                Initializing login...
              </Text>
            ) : (
              <>
                <Button
                  onClick={startLogin}
                  variant="primary"
                  size="lg"
                  className="w-full justify-center"
                >
                  Sign up
                </Button>
                <hr className="border-border-subtle" />
                <Text as="h2" variant="h2" weight="stronger">
                  Already have an account?
                </Text>
                <Button
                  onClick={startLogin}
                  variant="secondary"
                  size="lg"
                  className="w-full justify-center"
                >
                  Sign in
                </Button>
              </>
            )}

            {errorMessage ? (
              <Button
                onClick={retry}
                variant="secondary"
                size="lg"
                className="w-full justify-center"
              >
                Retry
              </Button>
            ) : null}
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
            src="/static/images/oss-hero.png"
            alt=""
            className="absolute inset-0 h-full w-full object-cover"
          />
        </div>
      </div>
    </div>
  );
};
