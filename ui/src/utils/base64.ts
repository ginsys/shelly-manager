export function bytesToBase64(bytes: Uint8Array): string {
  const output: string[] = []
  const chunkSize = 0x6000 // divisible by three
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, Math.min(offset + chunkSize, bytes.length))
    let binary = ''
    for (let index = 0; index < chunk.length; index++) {
      binary += String.fromCharCode(chunk[index])
    }
    output.push(btoa(binary))
  }
  return output.join('')
}
