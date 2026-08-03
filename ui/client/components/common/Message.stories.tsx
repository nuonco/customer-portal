import { Message, WarningMessage } from './Message'

export default {
  title: 'Common/Message',
  component: Message,
}

export const Default = () => (
  <Message theme="warning" title="Warning:">
    This should only be used in cases where an install was broken in an
    unusual way and needs to be manually removed.
  </Message>
)

export const Success = () => (
  <Message theme="success" title="Success:">
    Install link created successfully and is ready to share with customers.
  </Message>
)

export const Alert = () => (
  <Message theme="alert" title="Alert:">
    Failed to create install link. Please review inputs and try again.
  </Message>
)

export const Info = () => (
  <Message theme="info" title="Info:">
    This import may take a minute while we synchronize external data.
  </Message>
)

export const CustomTitle = () => (
  <Message theme="warning" title="Heads up:">
    This action will trigger a full re-sync and can take a few minutes.
  </Message>
)

export const WithoutTitle = () => (
  <Message theme="warning" title={null}>
    Proceed carefully. This operation cannot be rolled back.
  </Message>
)

export const RichContent = () => (
  <Message theme="warning">
    <span className="block">
      This action may affect active customer environments.
    </span>
    <span className="block mt-1">
      Confirm all related resources are deprovisioned first.
    </span>
  </Message>
)

export const BackwardCompatibleWarningMessage = () => (
  <WarningMessage>
    Existing WarningMessage imports continue to render a warning alert.
  </WarningMessage>
)