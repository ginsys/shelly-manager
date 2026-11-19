// The error code and message of a failed API response: the server's {success:false, error:{code,message}}
// envelope. A request made with responseType 'blob' (the downloads) delivers even an error body as a
// Blob, so a Blob is read as JSON text first. Anything that is not that envelope has no code or message.

function blobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') return blob.text()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(blob)
  })
}

async function errorEnvelope(data: unknown): Promise<{ code?: unknown; message?: unknown } | undefined> {
  let body = data
  if (typeof Blob !== 'undefined' && data instanceof Blob) {
    try {
      body = JSON.parse(await blobText(data))
    } catch {
      return undefined
    }
  }
  return (body as { error?: { code?: unknown; message?: unknown } } | null | undefined)?.error ?? undefined
}

export async function errorEnvelopeCode(data: unknown): Promise<string | undefined> {
  const code = (await errorEnvelope(data))?.code
  return typeof code === 'string' ? code : undefined
}

export async function errorEnvelopeMessage(data: unknown): Promise<string | undefined> {
  const message = (await errorEnvelope(data))?.message
  return typeof message === 'string' && message !== '' ? message : undefined
}
