import { Card } from "@/components/common/Card";
import { Markdown } from "@/components/common/Markdown/Markdown";
import { EmptyTabState } from "@/components/customer/install/EmptyTabState";
import { useSelectedInstall } from "./SelectedInstallProvider";

export const CustomerInstallReadmeView = () => {
  const { installDetail } = useSelectedInstall();

  if (!installDetail.readme.markdown) {
    return <EmptyTabState message="No README content is available for this app." />;
  }

  return (
    <Card className="bg-surface">
      <Markdown content={installDetail.readme.markdown} mode="app" />
    </Card>
  );
};
