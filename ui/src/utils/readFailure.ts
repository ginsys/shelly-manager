import { errorEnvelopeMessage } from '../api/errorBody'

// What the detail page shows for one live read that failed. The server's own message is used when
// there is one: a device that did not answer comes back as 504 whose message names the device and its
// address. No answer from the server at all (the browser gave up waiting) is said as that.
export async function describeReadFailure(err: unknown, what: string): Promise<string> {
  const e = err as { isAxiosError?: boolean; response?: { data?: unknown }; message?: string } | null
  if (e?.isAxiosError) {
    if (e.response) {
      return (await errorEnvelopeMessage(e.response.data)) ?? `Could not read ${what}`
    }
    return `Could not read ${what}: the server did not answer in time`
  }
  return e?.message || `Could not read ${what}`
}
