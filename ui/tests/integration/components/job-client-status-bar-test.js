/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { create } from 'ember-cli-page-object';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import sinon from 'sinon';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import jobClientStatusBar from 'nomad-ui/tests/pages/components/job-client-status-bar';

const JobClientStatusBar = create(jobClientStatusBar());

module('Integration | Component | job-client-status-bar', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    onSliceClick: sinon.spy(),
    job: {
      namespace: {
        get: () => 'my-namespace',
      },
    },
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
        unknown: [],
      },
    },
    isNarrow: true,
  });

  const commonTemplate = hbs`
    <JobClientStatusBar
      @onSliceClick={{onSliceClick}}
      @job={{job}}
      @jobClientStatus={{jobClientStatus}}
      @isNarrow={{isNarrow}}
    />`;

  test('it renders', async function (assert) {
    assert.expect(2);

    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);

    assert.ok(JobClientStatusBar.isPresent, 'Client Status Bar is rendered');
    await componentA11yAudit(this.element, assert);
  });

  test('it fires the onBarClick handler method when clicking a bar in the chart', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);
    await JobClientStatusBar.slices[0].click();
    assert.ok(props.onSliceClick.calledOnce);
  });

  test('it handles an update to client status property', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);
    await render(commonTemplate);
    const newProps = {
      ...props,
      jobClientStatus: {
        ...props.jobClientStatus,
        byStatus: {
          ...props.jobClientStatus.byStatus,
          starting: [],
          running: ['someNodeId'],
        },
      },
    };
    this.setProperties(newProps);
    await JobClientStatusBar.visitSlice('running');
    assert.ok(props.onSliceClick.calledOnce);
  });
});
