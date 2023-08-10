/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { decode } from 'nomad-ui/utils/stream-frames';
import { TextEncoderLite } from 'text-encoder-lite';
import base64js from 'base64-js';

const Encoder = new TextEncoderLite('utf-8');
const encode = (str) => base64js.fromByteArray(Encoder.encode(str));

module('Unit | Util | stream-frames', function () {
  const { btoa } = window;
  const decodeTestCases = [
    {
      name: 'Single frame',
      in: `{"Offset":100,"Data":"${btoa('Hello World')}"}`,
      out: {
        offset: 100,
        message: 'Hello World',
      },
    },
    {
      name: 'Multiple frames',
      // prettier-ignore
      in: `{"Offset":1,"Data":"${btoa('One fish,')}"}{"Offset":2,"Data":"${btoa( ' Two fish.')}"}{"Offset":3,"Data":"${btoa(' Red fish, ')}"}{"Offset":4,"Data":"${btoa('Blue fish.')}"}`,
      out: {
        offset: 4,
        message: 'One fish, Two fish. Red fish, Blue fish.',
      },
    },
    {
      name: 'Empty frames',
      in: '{}{}{}',
      out: {},
    },
    {
      name: 'Empty string',
      in: '',
      out: {},
    },
    {
      name: 'Multi-byte unicode',
      in: `{"Offset":1,"Data":"${encode('ワンワン 🐶')}"}`,
      out: {
        offset: 1,
        message: 'ワンワン 🐶',
      },
    },
  ];

  decodeTestCases.forEach((testCase) => {
    test(`decode: ${testCase.name}`, function (assert) {
      assert.deepEqual(decode(testCase.in), testCase.out);
    });
  });
});
