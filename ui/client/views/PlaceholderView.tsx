import { Text } from "@/components/common/Text";

interface IPlaceholderViewProps {
  title: string;
}

export const PlaceholderView = ({ title }: IPlaceholderViewProps) => (
  <section className="mx-auto flex max-w-4xl flex-col gap-4 p-6 md:p-8">
    <Text as="h1" variant="h2" weight="stronger">
      {title}
    </Text>
    <Text variant="body" theme="neutral">
      Coming soon.
    </Text>
  </section>
);
