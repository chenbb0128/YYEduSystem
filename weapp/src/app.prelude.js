;(function () {
  function encodeUtf8(input) {
    const source = String(input ?? '')
    const bytes = []

    for (let index = 0; index < source.length; index += 1) {
      const first = source.charCodeAt(index)
      let codePoint = first

      if (first >= 0xD800 && first <= 0xDBFF) {
        const second = source.charCodeAt(index + 1)
        if (second >= 0xDC00 && second <= 0xDFFF) {
          codePoint = 0x10000 + ((first - 0xD800) * 0x400) + (second - 0xDC00)
          index += 1
        }
        else {
          codePoint = 0xFFFD
        }
      }
      else if (first >= 0xDC00 && first <= 0xDFFF) {
        codePoint = 0xFFFD
      }

      if (codePoint <= 0x7F) {
        bytes.push(codePoint)
      }
      else if (codePoint <= 0x7FF) {
        bytes.push(0xC0 | (codePoint >> 6), 0x80 | (codePoint & 0x3F))
      }
      else if (codePoint <= 0xFFFF) {
        bytes.push(0xE0 | (codePoint >> 12), 0x80 | ((codePoint >> 6) & 0x3F), 0x80 | (codePoint & 0x3F))
      }
      else {
        bytes.push(0xF0 | (codePoint >> 18), 0x80 | ((codePoint >> 12) & 0x3F), 0x80 | ((codePoint >> 6) & 0x3F), 0x80 | (codePoint & 0x3F))
      }
    }

    return new Uint8Array(bytes)
  }

  function toBytes(input) {
    if (!input) {
      return new Uint8Array()
    }

    if (input instanceof ArrayBuffer) {
      return new Uint8Array(input)
    }

    if (typeof SharedArrayBuffer !== 'undefined' && input instanceof SharedArrayBuffer) {
      return new Uint8Array(input)
    }

    if (ArrayBuffer.isView(input)) {
      return new Uint8Array(input.buffer, input.byteOffset, input.byteLength)
    }

    return new Uint8Array()
  }

  function decodeUtf8(input) {
    const bytes = toBytes(input)
    let output = ''

    for (let index = 0; index < bytes.length;) {
      const first = bytes[index]

      if (first < 0x80) {
        output += String.fromCharCode(first)
        index += 1
        continue
      }

      if (first >= 0xC2 && first <= 0xDF && index + 1 < bytes.length) {
        const second = bytes[index + 1]
        if ((second & 0xC0) === 0x80) {
          output += String.fromCharCode(((first & 0x1F) << 6) | (second & 0x3F))
          index += 2
          continue
        }
      }

      if (first >= 0xE0 && first <= 0xEF && index + 2 < bytes.length) {
        const second = bytes[index + 1]
        const third = bytes[index + 2]
        if ((second & 0xC0) === 0x80 && (third & 0xC0) === 0x80 && (first !== 0xE0 || second >= 0xA0) && (first !== 0xED || second < 0xA0)) {
          output += String.fromCharCode(((first & 0x0F) << 12) | ((second & 0x3F) << 6) | (third & 0x3F))
          index += 3
          continue
        }
      }

      if (first >= 0xF0 && first <= 0xF4 && index + 3 < bytes.length) {
        const second = bytes[index + 1]
        const third = bytes[index + 2]
        const fourth = bytes[index + 3]
        if ((second & 0xC0) === 0x80 && (third & 0xC0) === 0x80 && (fourth & 0xC0) === 0x80 && (first !== 0xF0 || second >= 0x90) && (first !== 0xF4 || second < 0x90)) {
          const codePoint = ((first & 0x07) << 18) | ((second & 0x3F) << 12) | ((third & 0x3F) << 6) | (fourth & 0x3F)
          const adjusted = codePoint - 0x10000
          output += String.fromCharCode(0xD800 + (adjusted >> 10), 0xDC00 + (adjusted & 0x3FF))
          index += 4
          continue
        }
      }

      output += '\uFFFD'
      index += 1
    }

    return output
  }

  if (typeof globalThis.TextEncoder !== 'function') {
    globalThis.TextEncoder = function TextEncoder() {}
    globalThis.TextEncoder.prototype.encoding = 'utf-8'
    globalThis.TextEncoder.prototype.encode = function (input) {
      return encodeUtf8(input)
    }
    globalThis.TextEncoder.prototype.encodeInto = function (input, destination) {
      const source = encodeUtf8(input)
      const written = Math.min(source.length, destination.length)
      destination.set(source.subarray(0, written))
      return {
        read: String(input ?? '').length,
        written,
      }
    }
  }

  if (typeof globalThis.TextDecoder !== 'function') {
    globalThis.TextDecoder = function TextDecoder() {}
    globalThis.TextDecoder.prototype.encoding = 'utf-8'
    globalThis.TextDecoder.prototype.fatal = false
    globalThis.TextDecoder.prototype.ignoreBOM = false
    globalThis.TextDecoder.prototype.decode = function (input) {
      return decodeUtf8(input)
    }
  }
})()
