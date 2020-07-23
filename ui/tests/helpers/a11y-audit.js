import a11yAudit from 'ember-a11y-testing/test-support/audit';

export default async function defaultA11yAudit(...overriddenRules) {
  const rules = {
    'color-contrast': {
      enabled: false
    },
    'heading-order': {
      enabled: false
    }
  };

  overriddenRules.forEach(rule => rules[rule] = { enabled: false });

  await a11yAudit({rules});
}
