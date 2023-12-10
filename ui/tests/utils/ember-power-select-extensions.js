/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { click, settled } from '@ember/test-helpers';
// TODO: Contribute these helpers back upstream once we are on the latest version of
// ember-power-select (4.x)
//
// selectOpen and selectOpenChoose splits the existing selectChoose helper into two pieces
//   - selectOpen: open the select (await settled)
//   - selectOpenChoose: choose an option (await settled)
// Since the composite helper has two `await setted`s in it, the log changing tests can't use
// them. These tests require a run.later(run, run.cancelTimers, ms) to be inserted between
// these two moments. Doing it before opening means hanging on open not on select. Doing it
// after means hanging after the select has occurred (too late).
async function openIfClosedAndGetContentId(trigger) {
  let contentId =
    trigger.attributes['aria-owns'] &&
    `${trigger.attributes['aria-owns'].value}`;
  let content = contentId ? document.querySelector(`#${contentId}`) : undefined;
  // If the dropdown is closed, open it
  if (
    !content ||
    content.classList.contains('ember-basic-dropdown-content-placeholder')
  ) {
    await click(trigger);

    contentId = `${trigger.attributes['aria-owns'].value}`;
  }
  return contentId;
}

export async function selectOpen(cssPathOrTrigger) {
  let trigger;
  if (cssPathOrTrigger instanceof HTMLElement) {
    if (cssPathOrTrigger.classList.contains('ember-power-select-trigger')) {
      trigger = cssPathOrTrigger;
    } else {
      trigger = cssPathOrTrigger.querySelector('.ember-power-select-trigger');
    }
  } else {
    trigger = document.querySelector(
      `${cssPathOrTrigger} .ember-power-select-trigger`
    );

    if (!trigger) {
      trigger = document.querySelector(cssPathOrTrigger);
    }

    if (!trigger) {
      throw new Error(
        `You called "selectOpen('${cssPathOrTrigger}')" but no select was found using selector "${cssPathOrTrigger}"`
      );
    }
  }

  if (trigger.scrollIntoView) {
    trigger.scrollIntoView();
  }

  return await openIfClosedAndGetContentId(trigger);
}

export async function selectOpenChoose(
  contentId,
  valueOrSelector,
  optionIndex
) {
  let target;
  // Select the option with the given text
  let options = document.querySelectorAll(
    `#${contentId} .ember-power-select-option`
  );
  let potentialTargets = [].slice
    .apply(options)
    .filter((opt) => opt.textContent.indexOf(valueOrSelector) > -1);
  if (potentialTargets.length === 0) {
    potentialTargets = document.querySelectorAll(
      `#${contentId} ${valueOrSelector}`
    );
  }
  if (potentialTargets.length > 1) {
    let filteredTargets = [].slice
      .apply(potentialTargets)
      .filter((t) => t.textContent.trim() === valueOrSelector);
    if (optionIndex === undefined) {
      target = filteredTargets[0] || potentialTargets[0];
    } else {
      target = filteredTargets[optionIndex] || potentialTargets[optionIndex];
    }
  } else {
    target = potentialTargets[0];
  }
  if (!target) {
    throw new Error(
      `You called "selectOpenChoose('${valueOrSelector}')" but "${valueOrSelector}" didn't match any option`
    );
  }
  await click(target);
  return settled();
}
