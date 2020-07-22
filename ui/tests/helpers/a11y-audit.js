import a11yAudit from 'ember-a11y-testing/test-support/audit';

export default async function defaultA11yAudit() {
  await a11yAudit({
    rules: {
      'button-name': {
        enabled: false
      },
      'color-contrast': {
        enabled: false
      },
      'heading-order': {
        enabled: false
      },
      'label': {
        enabled: false
      },
      'scrollable-region-focusable': {
        enabled: false
      },
    }
  });
}
