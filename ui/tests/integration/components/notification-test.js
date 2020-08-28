import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { STATUS_CLASSES } from 'nomad-ui/models/deployment';

module('Integration | Component | notification', function(hooks) {
  setupRenderingTest(hooks);

  STATUS_CLASSES.forEach(statusClass => {
    test(`the ${statusClass} notification passes an accessibility audit`, async function(assert) {
      this.statusClass = statusClass;
      await render(hbs`
        <div class="notification {{statusClass}}">
          notification text
        </div>
      `);

      await componentA11yAudit(this.element, assert);
    });
  });
});
