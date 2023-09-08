/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Log Stream',
};

export let LogStream = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Log stream</h5>
      <div class="boxed-section">
        <div class="boxed-section-head">
          <span>
            <button
              class="button {{if (eq mode1 "stdout") "is-info"}}"
              onclick={{action (mut mode1) "stdout"}}>stdout</button>
            <button
              class="button {{if (eq mode1 "stderr") "is-danger"}}"
              onclick={{action (mut mode1) "stderr"}}>stderr</button>
          </span>
          <span class="pull-right">
            <button class="button is-white">Head</button>
            <button class="button is-white">Tail</button>
            <button class="button is-white" onclick={{toggle "isPlaying1" this}}>
              {{x-icon (if isPlaying1 "media-play" "media-pause") class="is-text"}}
            </button>
          </span>
        </div>
        <div class="boxed-section-body is-dark is-full-bleed">
          <pre class="cli-window"><code>{{if (eq mode1 "stdout") sampleOutput sampleError}}</code></pre>
        </div>
      </div>
      `,
    context: {
      mode1: 'stdout',
      isPlaying1: true,

      sampleOutput: `Sample output
> 1
> 2
> 3
[00:12:58] Log output here
[00:15:29] [ERR] Uh oh
Loading.
Loading..
Loading...

  >> Done! <<

    `,

      sampleError: `Sample error

[====|--------------------] 20%

!!! Unrecoverable error:

  Cannot continue beyond this point. Exception should be caught.
  This is not a mistake. You did something wrong. Check the code.
  No, you will not receive any more details or guidance from this
  error message.

    `,
    },
  };
};
