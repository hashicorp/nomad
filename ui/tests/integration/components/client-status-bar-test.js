import { module, test } from 'qunit';
import { create } from 'ember-cli-page-object';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import sinon from 'sinon';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import clientStatusBar from 'nomad-ui/tests/pages/components/client-status-bar';

const ClientStatusBar = create(clientStatusBar());

module('Integration | Component | client-status-bar', function(hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    onBarClick: sinon.spy(),
    jobClientStatus: {
      byStatus: {
        queued: [],
        starting: ['someNodeId'],
        running: [],
        complete: [],
        degraded: [],
        failed: [],
        lost: [],
        notScheduled: [],
      },
    },
    isNarrow: true,
  });

  const commonTemplate = hbs`
    <ClientStatusBar 
      @onBarClick={{onBarClick}} 
      @jobClientStatus={{jobClientStatus}} 
      @isNarrow={{isNarrow}}
    />`;

  test('it renders', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(ClientStatusBar.isPresent, 'Client Status Bar is rendered');
    await componentA11yAudit(this.element, assert);
  });

  test('it fires the onBarClick handler method when clicking a bar in the chart', async function(assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);
    await ClientStatusBar.visitBar('starting');
    assert.ok(props.onBarClick.calledOnce);
  });
});
