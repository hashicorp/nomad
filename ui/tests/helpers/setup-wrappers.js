import { setupApplicationTest as qunitSetupApplicationTest, setupRenderingTest as qunitSetupRenderingTest } from 'ember-qunit';

import config from 'nomad-ui/config/environment';
import sinon from 'sinon';

function setupApplicationTest(hooks) {
  if (config.percy.enabled) {
    hooks.beforeEach(() => {
      sinon.useFakeTimers({ shouldAdvanceTime: true });
    });
  }

  qunitSetupApplicationTest(...arguments);
}

function setupRenderingTest(hooks) {
  if (config.percy.enabled) {
    hooks.beforeEach(() => {
      sinon.useFakeTimers({ shouldAdvanceTime: true });
    });
  }

  qunitSetupRenderingTest(...arguments);
}

export {
  setupApplicationTest,
  setupRenderingTest
};
