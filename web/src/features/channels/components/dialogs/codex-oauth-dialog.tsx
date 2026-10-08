import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useId, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  completeCodexAuthorization,
  startCodexAuthorization,
  type CodexAuthorizationResult,
} from '../../api'
import {
  codexAuthorizationSchema,
  type CodexAuthorizationFormValues,
} from '../../lib/codex-authorization'

type CodexOAuthDialogProps = {
  open: boolean
  channelId?: number
  proxy?: string
  onOpenChange: (open: boolean) => void
  onAuthorized: (result: CodexAuthorizationResult) => void
}

export function CodexOAuthDialog(props: CodexOAuthDialogProps) {
  const { t } = useTranslation()
  const formID = useId()
  const form = useForm<CodexAuthorizationFormValues>({
    resolver: zodResolver(codexAuthorizationSchema),
    defaultValues: { input: '' },
  })
  const resetAuthorizationForm = form.reset
  const input = form.watch('input')
  const [authorizeURL, setAuthorizeURL] = useState('')
  const [error, setError] = useState('')
  const [starting, setStarting] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [attempt, setAttempt] = useState(0)
  const revisionRef = useRef(0)
  const busyRef = useRef(false)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    const revision = ++revisionRef.current
    setAuthorizeURL('')
    resetAuthorizationForm({ input: '' })
    setError('')
    setSubmitting(false)
    busyRef.current = false
    if (!props.open) return
    const abort = new AbortController()
    abortRef.current = abort
    setStarting(true)
    void startCodexAuthorization(props.channelId, props.proxy, abort.signal)
      .then((result) => {
        if (revisionRef.current === revision) {
          setAuthorizeURL(result.authorize_url)
        }
      })
      .finally(() => {
        if (revisionRef.current === revision) setStarting(false)
      })
      .catch((failure: unknown) => {
        if (revisionRef.current === revision) {
          setError(
            getServerErrorMessage(
              failure,
              t('Failed to start Codex authorization')
            )
          )
        }
      })
    return () => {
      revisionRef.current = revision + 1
      abort.abort()
    }
  }, [
    props.open,
    props.channelId,
    props.proxy,
    attempt,
    t,
    resetAuthorizationForm,
  ])

  const complete = async (values: CodexAuthorizationFormValues) => {
    if (busyRef.current || !authorizeURL) return
    busyRef.current = true
    setSubmitting(true)
    setError('')
    const revision = revisionRef.current
    try {
      const result = await completeCodexAuthorization(
        values.input,
        props.channelId,
        abortRef.current?.signal
      )
      if (revisionRef.current !== revision) return
      props.onAuthorized(result)
      props.onOpenChange(false)
    } catch (failure) {
      if (revisionRef.current !== revision) return
      form.setError('input', {
        type: 'server',
        message: getServerErrorMessage(
          failure,
          t('Codex authorization failed')
        ),
      })
      setAuthorizeURL('')
    } finally {
      if (revisionRef.current === revision) {
        busyRef.current = false
        setSubmitting(false)
      }
    }
  }

  let submitLabel = t('Generate credential')
  if (props.channelId) submitLabel = t('Save authorized credential')
  if (submitting) submitLabel = t('Authorizing...')

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Authorize Codex account')}
      description={t(
        'Open the authorization page, sign in to your upstream account, then paste the complete callback URL. The localhost page may not load; copy its address anyway. Authorization expires in ten minutes.'
      )}
      contentClassName='sm:max-w-xl'
      footerClassName='flex-wrap'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          {!authorizeURL && !starting && (
            <Button onClick={() => setAttempt((value) => value + 1)}>
              {t('Restart authorization')}
            </Button>
          )}
          <Button
            type='submit'
            form={formID}
            disabled={!authorizeURL || !input.trim() || starting || submitting}
          >
            {submitLabel}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={formID}
          onSubmit={form.handleSubmit(complete)}
          className='space-y-4'
        >
          {starting && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t('Creating authorization link...')}
            </p>
          )}
          {authorizeURL && (
            <div className='space-y-2'>
              <Label htmlFor='codex-authorization-link'>
                {t('Authorization link')}
              </Label>
              <Input
                id='codex-authorization-link'
                value={authorizeURL}
                readOnly
                className='font-mono text-xs'
              />
              <div className='flex flex-wrap gap-2'>
                <Button
                  role='link'
                  render={
                    <a
                      href={authorizeURL}
                      target='_blank'
                      rel='noopener noreferrer'
                    />
                  }
                >
                  {t('Open authorization page')}
                </Button>
                <CopyButton
                  value={authorizeURL}
                  variant='outline'
                  size='default'
                  aria-label={t('Copy authorization link')}
                >
                  {t('Copy authorization link')}
                </CopyButton>
              </div>
            </div>
          )}
          <FormField
            control={form.control}
            name='input'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Authorization callback')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    rows={3}
                    disabled={starting || submitting || !authorizeURL}
                    placeholder='http://localhost:1455/auth/callback?code=…&state=…'
                    autoComplete='off'
                  />
                </FormControl>
                <FormMessage role='alert' />
              </FormItem>
            )}
          />
          {error && (
            <Alert variant='destructive'>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
        </form>
      </Form>
    </Dialog>
  )
}
