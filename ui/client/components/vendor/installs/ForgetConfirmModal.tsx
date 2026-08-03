import { useState } from 'react'
import { Button } from '@/components/common/Button'
import { Input } from '@/components/common/form/Input'
import { WarningMessage } from '@/components/common/Message'
import { ModalBase } from '@/components/surfaces/Modal'
import { Text } from '@/components/common/Text'
import { forgetInstall, type TAdminInstall } from '@/lib/api/vendor/get-installs'

interface IForgetConfirmModal {
  install: TAdminInstall
  orgId: string
  onClose: () => void
  onForgotten: (installId: string) => void
}

export const ForgetConfirmModal = ({
  install,
  orgId,
  onClose,
  onForgotten,
}: IForgetConfirmModal) => {
  const [confirmName, setConfirmName] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const isValid = confirmName === install.Name

  const handleForget = async () => {
    setIsSubmitting(true)
    setError(null)
    try {
      await forgetInstall(orgId, install.ID)
      onForgotten(install.ID)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to forget install')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ModalBase
      isVisible
      onClose={onClose}
      heading={`Forget ${install.Name}`}
      showFooter={false}
      size="sm"
      className="max-w-md"
    >
        <div className="flex flex-col gap-4">
          <WarningMessage>
            This should only be used in cases where an install was broken in an
            unordinary way and needs to be manually removed.
          </WarningMessage>
          <Text variant="body">
            This action will remove the install and cannot be undone. You should
            only do this after successfully deprovisioning the install and its
            CloudFormation stack.
          </Text>
          <Input
            id="forget-install-confirm-name"
            type="text"
            value={confirmName}
            onChange={(e) => setConfirmName(e.target.value)}
            placeholder={install.Name}
            labelProps={{
              labelText: (
                <>
                  Type{' '}
                  <code className="font-medium text-red-600 dark:text-red-400">
                    {install.Name}
                  </code>{' '}
                  to confirm:
                </>
              ),
            }}
            className="focus:ring-red-500! focus:border-red-500!"
          />
          {error ? (
            <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
          ) : null}
          <div className="flex justify-end">
            <Button
              variant="danger"
              disabled={!isValid || isSubmitting}
              onClick={() => void handleForget()}
            >
              {isSubmitting ? 'Forgetting...' : 'Forget install'}
            </Button>
          </div>
        </div>
    </ModalBase>
  )
}
