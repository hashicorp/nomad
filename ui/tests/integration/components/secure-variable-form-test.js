import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { click, findAll, render } from '@ember/test-helpers';

module('Integration | Component | secure-variable-form', function (hooks) {
  setupRenderingTest(hooks);

  test('passes an accessibility audit', async function (assert) {
    assert.expect(1);
    await render(hbs`<SecureVariableForm />`);
    await componentA11yAudit(this.element, assert);
  });

  test('shows a single row by default and expands on "Add More"', async function (assert) {
    // assert.expect(6);

    await render(hbs`<SecureVariableForm />`);

    assert.equal(
      findAll('div.key-value').length,
      1,
      'A single KV row exists by default'
    );

    await click('.key-value button');

    assert.equal(
      findAll('div.key-value').length,
      2,
      'A second KV row exists after adding a new one'
    );

    await click('.key-value button');

    assert.equal(
      findAll('div.key-value').length,
      3,
      'A third KV row exists after adding a new one'
    );
  });
});
