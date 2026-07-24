export function parseStrictJSON(text: string, maxDepth = 64): unknown {
  let at = 0

  const space = () => {
    while (at < text.length && (text[at] === ' ' || text[at] === '\n' || text[at] === '\r' || text[at] === '\t')) at++
  }
  const consume = (expected: string) => {
    if (text[at] === expected) {
      at++
      return true
    }
    return false
  }
  const parseString = (): string => {
    const start = at
    at++
    while (at < text.length) {
      const code = text.charCodeAt(at)
      if (text[at] === '"') {
        at++
        return decodeJSONString(text.slice(start, at))
      }
      if (code < 0x20) throw new Error('unescaped control character')
      if (text[at] !== '\\') {
        if (code >= 0xd800 && code <= 0xdbff) {
          const low = text.charCodeAt(at + 1)
          if (!(low >= 0xdc00 && low <= 0xdfff)) throw new Error('lone high surrogate')
          at += 2
        } else if (code >= 0xdc00 && code <= 0xdfff) {
          throw new Error('lone low surrogate')
        } else {
          at++
        }
        continue
      }
      at++
      const escape = text[at]
      if ('"\\/bfnrt'.includes(escape)) {
        at++
        continue
      }
      if (escape !== 'u') throw new Error('invalid string escape')
      const high = readEscape()
      if (high >= 0xd800 && high <= 0xdbff) {
        if (text[at] !== '\\' || text[at + 1] !== 'u') throw new Error('lone high surrogate')
        at++
        const low = readEscape()
        if (low < 0xdc00 || low > 0xdfff) throw new Error('invalid surrogate pair')
      } else if (high >= 0xdc00 && high <= 0xdfff) {
        throw new Error('lone low surrogate')
      }
    }
    throw new Error('unterminated string')
  }
  const readEscape = (): number => {
    const digits = text.slice(at + 1, at + 5)
    if (!/^[0-9a-fA-F]{4}$/.test(digits)) throw new Error('invalid Unicode escape')
    at += 5
    return Number.parseInt(digits, 16)
  }
  const parseNumber = (): number => {
    const start = at
    if (consume('-') && at >= text.length) throw new Error('incomplete number')
    if (consume('0')) {
      if (/[0-9]/.test(text[at] ?? '')) throw new Error('leading zero')
    } else {
      if (!/[1-9]/.test(text[at] ?? '')) throw new Error('invalid number')
      while (/[0-9]/.test(text[at] ?? '')) at++
    }
    if (consume('.')) {
      if (!/[0-9]/.test(text[at] ?? '')) throw new Error('fraction requires digits')
      while (/[0-9]/.test(text[at] ?? '')) at++
    }
    if (text[at] === 'e' || text[at] === 'E') {
      at++
      if (text[at] === '+' || text[at] === '-') at++
      if (!/[0-9]/.test(text[at] ?? '')) throw new Error('exponent requires digits')
      while (/[0-9]/.test(text[at] ?? '')) at++
    }
    const raw = text.slice(start, at)
    const value = Number(raw)
    if (!Number.isFinite(value)) throw new Error('number is outside binary64')
    if (Object.is(value, 0) || Object.is(value, -0)) {
      const mantissa = raw.replace(/^-/, '').split(/[eE]/)[0].replace('.', '').replace(/^0+/, '')
      if (mantissa !== '') throw new Error('number underflows to zero')
    }
    if (Number.isInteger(value) && !Number.isSafeInteger(value)) throw new Error('integer is outside the safe range')
    return value
  }
  const literal = (word: string, value: unknown): unknown => {
    if (text.slice(at, at + word.length) !== word) throw new Error(`invalid literal at ${at}`)
    at += word.length
    return value
  }
  const value = (depth: number): unknown => {
    if (text[at] === '{') return object(depth + 1)
    if (text[at] === '[') return array(depth + 1)
    if (text[at] === '"') return parseString()
    if (text[at] === 't') return literal('true', true)
    if (text[at] === 'f') return literal('false', false)
    if (text[at] === 'n') return literal('null', null)
    if (text[at] === '-' || /[0-9]/.test(text[at] ?? '')) return parseNumber()
    throw new Error(`unexpected token at ${at}`)
  }
  const object = (depth: number): Record<string, unknown> => {
    if (depth > maxDepth) throw new Error(`maximum JSON depth ${maxDepth} exceeded`)
    at++
    space()
    const result = Object.create(null) as Record<string, unknown>
    if (consume('}')) return result
    while (true) {
      if (text[at] !== '"') throw new Error('object name must be a string')
      const name = parseString()
      if (Object.prototype.hasOwnProperty.call(result, name)) throw new Error(`duplicate object name ${name}`)
      space()
      if (!consume(':')) throw new Error('missing colon')
      space()
      result[name] = value(depth)
      space()
      if (consume('}')) return result
      if (!consume(',')) throw new Error('missing comma')
      space()
    }
  }
  const array = (depth: number): unknown[] => {
    if (depth > maxDepth) throw new Error(`maximum JSON depth ${maxDepth} exceeded`)
    at++
    space()
    const result: unknown[] = []
    if (consume(']')) return result
    while (true) {
      result.push(value(depth))
      space()
      if (consume(']')) return result
      if (!consume(',')) throw new Error('missing comma')
      space()
    }
  }

  space()
  const parsed = value(0)
  space()
  if (at !== text.length) throw new Error('trailing JSON value')
  return parsed
}

function decodeJSONString(token: string): string {
  let result = ''
  for (let index = 1; index < token.length - 1; index++) {
    if (token[index] !== '\\') {
      result += token[index]
      continue
    }
    const escape = token[++index]
    const simple: Record<string, string> = {
      '"': '"', '\\': '\\', '/': '/', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t',
    }
    if (escape !== 'u') {
      result += simple[escape]
      continue
    }
    const high = Number.parseInt(token.slice(index + 1, index + 5), 16)
    index += 4
    if (high >= 0xd800 && high <= 0xdbff) {
      index += 2 // skip the following \u
      const low = Number.parseInt(token.slice(index + 1, index + 5), 16)
      index += 4
      result += String.fromCodePoint(0x10000 + ((high - 0xd800) << 10) + (low - 0xdc00))
    } else {
      result += String.fromCharCode(high)
    }
  }
  return result
}
