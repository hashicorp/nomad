/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { clearRender, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | job-template-card', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders a job template card', async function (assert) {
    const TEST_NOOP = () => undefined;
    this.set('onChange', TEST_NOOP);
    await render(hbs`
      <JobTemplateCard @icon={{"activity"}} @label={{"Tomster"}} @description={{"Ready to start shipping?"}} @onChange={{this.onChange}} />
    `);

    assert
      .dom('[data-test-template-label]')
      .hasText('Tomster', 'We render the name of the template');

    assert
      .dom('[data-test-template-description]')
      .hasText(
        'Ready to start shipping?',
        'We render the name of the template'
      );

    assert
      .dom('svg.flight-icon.flight-icon-activity')
      .exists('We render the corresponding flight icon from HDS');

    clearRender();
    await render(hbs`
    <JobTemplateCard @label={{"Tomster"}} @description={{"Ready to start shipping?"}} @onChange={{this.onChange}} />
  `);
    assert
      .dom('svg.flight-icon')
      .doesNotExist(
        'We do not render the corresponding flight icon from HDS if icon arg not provided'
      );
  });
});
