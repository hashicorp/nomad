/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { TextDecoderLite } from 'text-encoder-lite';
import base64js from 'base64-js';

const decoder = new TextDecoderLite('utf-8');

/**
 *
 * @param {string} chunk
 * Chunk is an undelimited string of valid JSON objects as returned by a streaming endpoint.
 * Each JSON object in a chunk contains two properties:
 *   Offset {number} The index from the beginning of the stream at which this JSON object starts
 *   Data {string} A base64 encoded string representing the contents of the stream this JSON
 *                 object represents.
 */
export function decode(chunk) {
  const lines = chunk.replace(/\}\{/g, '}\n{').split('\n').without('');
  const frames = lines
    .map((line) => JSON.parse(line))
    .filter((frame) => frame.Data);

  if (frames.length) {
    frames.forEach((frame) => (frame.Data = b64decode(frame.Data)));
    return {
      offset: frames[frames.length - 1].Offset,
      message: frames.mapBy('Data').join(''),
    };
  }

  return {};
}

function b64decode(str) {
  return decoder.decode(base64js.toByteArray(str));
}
