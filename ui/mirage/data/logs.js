/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export const logFrames = [
  'hello world\n',
  'some more output\ngoes here\n\n--> potentially helpful',
  ' hopefully, at least.\n',
];

export const logEncode = (frames, index) => {
  return frames
    .slice(0, index + 1)
    .map(frame => window.btoa(frame))
    .map((frame, innerIndex) => {
      const offset = frames.slice(0, innerIndex).reduce((sum, frame) => sum + frame.length, 0);
      return JSON.stringify({ Offset: offset, Data: frame });
    })
    .join('');
};
