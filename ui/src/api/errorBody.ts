// The error code and message of a failed API response: the server's {success:false, error:{code,message}}
// envelope. A request made with responseType 'blob' (the downloads) delivers even an error body as a
// Blob, so a Blob is read as JSON text first. Anything that is not that envelope has no code or message.

export function blobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') return blob.text()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(blob)
  })
}

async function errorEnvelope(data: unknown): Promise<{ code?: unknown; message?: unknown; details?: unknown } | undefined> {
  let body = data
  if (typeof Blob !== 'undefined' && data instanceof Blob) {
    try {
      body = JSON.parse(await blobText(data))
    } catch {
      return undefined
    }
  }
  return (body as { error?: { code?: unknown; message?: unknown; details?: unknown } } | null | undefined)?.error ?? undefined
}

// The `details` of the envelope when it is a text. A validation refusal puts its reason there, and some
// routes send a map, which is not a text and is left out.
export async function errorEnvelopeDetails(data: unknown): Promise<string | undefined> {
  const details = (await errorEnvelope(data))?.details
  return typeof details === 'string' && details !== '' ? details : undefined
}

export async function errorEnvelopeCode(data: unknown): Promise<string | undefined> {
  const code = (await errorEnvelope(data))?.code
  return typeof code === 'string' ? code : undefined
}

export async function errorEnvelopeMessage(data: unknown): Promise<string | undefined> {
  const message = (await errorEnvelope(data))?.message
  return typeof message === 'string' && message !== '' ? message : undefined
}
