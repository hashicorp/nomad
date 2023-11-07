/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
// @ts-check
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { allScenarios } from '../../mirage/scenarios/default';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import percySnapshot from '@percy/ember';
import Actions from 'nomad-ui/tests/pages/jobs/job/actions';

module('Acceptance | actions', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
  });

  test('Actions show up on the Job Index page, permissions allowing', async function (assert) {
    assert.expect(8);
    allScenarios.smallCluster(server);
    let managementToken = server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    let clientReaderToken = server.create('token', {
      type: 'client',
      name: "N. O'DeReader",
    });

    const allocExecPolicy = server.create('policy', {
      id: 'alloc-exec',
      rules: `
        namespace "*" {
          policy = "read"
          capabilities = ["list-jobs", "alloc-exec", "read-logs"]
        }
      `,
      rulesJSON: {
        Namespaces: [
          {
            Capabilities: ['list-jobs', 'alloc-exec', 'read-logs'],
            Name: '*',
          },
        ],
      },
    });

    let allocExecToken = server.create('token', {
      type: 'client',
      name: 'Alloc Exec Token',
      policyIds: [allocExecPolicy.id],
    });

    await Actions.visitIndex({ id: 'actionable-job' });

    // no actions dropdown by default
    assert.notOk(Actions.hasTitleActions, 'No actions dropdown by default');
    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Actions.visitIndex({ id: 'actionable-job' });
    assert.ok(
      Actions.hasTitleActions,
      'Management token sees actions dropdown'
    );
    assert.ok(Actions.taskRowActions.length, 'Task row has actions dropdowns');

    await a11yAudit(assert);

    // Sign out and sign back in as a token without alloc exec
    await Tokens.visit();
    await Tokens.clear();
    await Tokens.secret(clientReaderToken.secretId).submit();
    await Actions.visitIndex({ id: 'actionable-job' });
    assert.notOk(
      Actions.hasTitleActions,
      'Basic client token does not see actions dropdown'
    );
    assert.notOk(
      Actions.taskRowActions.length,
      'Basic client token does not see task row actions dropdowns'
    );

    // Sign out and sign back in as a token with alloc exec
    await Tokens.visit();
    await Tokens.clear();
    await Tokens.secret(allocExecToken.secretId).submit();
    await Actions.visitIndex({ id: 'actionable-job' });
    assert.ok(
      Actions.hasTitleActions,
      'Alloc exec token sees actions dropdown'
    );
    assert.ok(
      Actions.taskRowActions.length,
      'Alloc exec token sees task row actions dropdowns'
    );
  });

  // Running actions test
  test('Running actions and notifications', async function (assert) {
    assert.expect(13);
    allScenarios.smallCluster(server);
    let managementToken = server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Actions.visitIndex({ id: 'actionable-job' });
    assert.ok(
      Actions.hasTitleActions,
      'Management token sees actions dropdown'
    );

    // Open the dropdown
    await Actions.titleActions.click();
    assert.equal(
      Actions.titleActions.actions.length,
      5,
      '5 actions show up in the dropdown'
    );

    assert.equal(
      Actions.titleActions.multiAllocActions.length,
      4,
      '4 actions in the dropdown have multiple allocs to run against'
    );
    assert.equal(
      Actions.titleActions.singleAllocActions.length,
      1,
      '1 action in the dropdown has a single alloc to run against'
    );

    assert.equal(
      Actions.titleActions.multiAllocActions[0].button[0].expanded,
      'false',
      "The first action's dropdown is not expanded"
    );
    assert.notOk(
      Actions.titleActions.multiAllocActions[0].showsDisclosureContent,
      "The first action's dropdown subcontent does not yet exist"
    );

    await Actions.titleActions.actions[0].click();
    assert.equal(
      Actions.titleActions.multiAllocActions[0].button[0].expanded,
      'true',
      "The first action's dropdown is expanded"
    );
    assert.ok(
      Actions.titleActions.multiAllocActions[0].showsDisclosureContent,
      "The first action's dropdown subcontent exists"
    );

    await percySnapshot(assert);

    // run on a random alloc
    await Actions.titleActions.multiAllocActions[0].subActions[0].click();

    assert.equal(
      Actions.toast.length,
      1,
      'A toast notification pops up upon running an action'
    );

    assert.ok(
      Actions.toast[0].code.includes('Message Received'),
      'The notification contains the message from the action'
    );
    assert.ok(
      Actions.toast[0].titleBar.includes('Finished'),
      'The notification contains the message from the action'
    );

    // run on all allocs
    await Actions.titleActions.multiAllocActions[0].subActions[1].click();

    assert.equal(
      Actions.toast.length,
      6,
      'Running on all allocs in the group (5) results in 6 total toasts'
    );

    // Click the orphan alloc action
    await Actions.titleActions.singleAllocActions[0].button[0].click();
    assert.equal(
      Actions.toast.length,
      7,
      'Running on an orphan alloc results in 1 further action/toast'
    );

    await percySnapshot(assert);
  });

  test('Running actions from a task row', async function (assert) {
    allScenarios.smallCluster(server);
    let managementToken = server.create('token', {
      type: 'management',
      name: 'Management Token',
    });

    await Tokens.visit();
    const { secretId } = managementToken;
    await Tokens.secret(secretId).submit();
    await Actions.visitAllocs({ id: 'actionable-job' });

    // Get the number of rows; each of them should have an actions dropdown
    const job = server.schema.jobs.find('actionable-job');
    const numberOfTaskRows = server.schema.allocations
      .all()
      .models.filter((a) => a.jobId === job.name)
      .map((a) => a.taskStates.models)
      .flat().length;

    assert.equal(
      Actions.taskRowActions.length,
      numberOfTaskRows,
      'Each task row has an actions dropdown'
    );
    await Actions.taskRowActions[0].click();

    assert.equal(
      Actions.taskRowActions[0].actions.length,
      1,
      'Actions within a task row actions dropdown are shown'
    );

    await Actions.taskRowActions[0].actions[0].click();
    assert.equal(
      Actions.toast.length,
      1,
      'A toast notification pops up upon running an action'
    );
    assert.ok(
      Actions.toast[0].code.includes('Message Received'),
      'The notification contains the message from the action'
    );
  });
});
