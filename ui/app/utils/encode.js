/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import base64js from 'base64-js';
import { TextDecoderLite, TextEncoderLite } from 'text-encoder-lite';

export { base64EncodeString, base64DecodeString };

function base64EncodeString(string) {
  if (!string) {
    string = '';
  }

  const encoded = new TextEncoderLite('utf-8').encode(string);
  return base64js.fromByteArray(encoded);
}

function base64DecodeString(b64String) {
  if (!b64String) {
    b64String = base64EncodeString('');
  }

  const uint8array = base64js.toByteArray(b64String);
  return new TextDecoderLite('utf-8').decode(uint8array);
}
