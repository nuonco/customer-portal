import React, { Component, ErrorInfo, ReactNode } from 'react'
import { Message } from '@/components/common/Message'

type FallbackRender = (props: { error: Error }) => ReactNode

interface ErrorBoundaryProps {
  children: ReactNode
  fallback?: ReactNode | FallbackRender
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    if (process.env.NODE_ENV !== 'production') {
      console.error('ErrorBoundary caught an error', error, info)
    }
  }

  render() {
    const { hasError, error } = this.state
    const { fallback, children } = this.props

    if (hasError) {
      if (typeof fallback === 'function') {
        return (fallback as FallbackRender)({ error: error! })
      }

      if (fallback) return fallback

      return (
        <Message className="p-4" title="Something went wrong.">
          <pre className="text-xs whitespace-pre-wrap">{error?.message}</pre>
        </Message>
      )
    }

    return children
  }
}
