import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-status/failed-or-lost', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    let allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    this.set('allocs', allocs);

    await render(hbs`<JobStatus::FailedOrLost
      @title="Rescheduled"
      @description="Rescheduled Allocations"
      @allocs={{this.allocs}}
    />`);

    assert.dom('h4').hasText('Rescheduled');
    assert.dom('.failed-or-lost-link').hasText('2');

    await this.pauseTest();
    allocs.push({
      id: 3,
      name: 'alloc3',
    });

    this.set('allocs', allocs);
    await this.pauseTest();

    assert.dom('.failed-or-lost-link').hasText('3');

    await componentA11yAudit(this.element, assert);
  });

  test('it links or does not link appropriately', async function (assert) {
    let allocs = [
      {
        id: 1,
        name: 'alloc1',
      },
      {
        id: 2,
        name: 'alloc2',
      },
    ];

    this.set('allocs', allocs);

    await render(hbs`<JobStatus::FailedOrLost
      @title="Rescheduled"
      @description="Rescheduled Allocations"
      @allocs={{this.allocs}}
    />`);

    // Ensure it's of type a
    assert.dom('.failed-or-lost-link').hasTagName('a');
    this.set('allocs', []);
    assert.dom('.failed-or-lost-link').doesNotHaveTagName('a');
  });
});
