/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  create,
  clickable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';
import { run } from '@ember/runloop';
import {
  selectOpen,
  selectOpenChoose,
} from '../../utils/ember-power-select-extensions';

export default create({
  visit: visitable('/servers/:name/monitor'),

  logsArePresent: isPresent('[data-test-log-box]'),

  error: {
    isShown: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  async selectLogLevel(level) {
    const contentId = await selectOpen('[data-test-level-switcher-parent]');
    run.later(run, run.cancelTimers, 500);
    await selectOpenChoose(contentId, level);
  },
});
