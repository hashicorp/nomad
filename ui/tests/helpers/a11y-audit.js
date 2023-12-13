/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import a11yAudit from 'ember-a11y-testing/test-support/audit';

function appendRuleOverrides(overriddenRules) {
  const rules = {
    'color-contrast': {
      enabled: false,
    },
    'heading-order': {
      enabled: false,
    },
  };

  overriddenRules.forEach((rule) => (rules[rule] = { enabled: false }));

  return rules;
}

export default async function defaultA11yAudit(assert, ...overriddenRules) {
  await a11yAudit({ rules: appendRuleOverrides(overriddenRules) });
  assert.ok(true, 'a11y audit passes');
}

export async function componentA11yAudit(element, assert, ...overriddenRules) {
  await a11yAudit(element, { rules: appendRuleOverrides(overriddenRules) });
  assert.ok(true, 'a11y audit passes');
}
