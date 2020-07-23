import a11yAudit from 'ember-a11y-testing/test-support/audit';

export default async function defaultA11yAudit() {
  await a11yAudit({
    rules: {
      'color-contrast': {
        enabled: false
      },
      'heading-order': {
        enabled: false
      },
      'scrollable-region-focusable': {
        enabled: false
      },
    }
  });
}
