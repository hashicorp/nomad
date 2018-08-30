import { run } from '@ember/runloop';
import { merge } from '@ember/polyfills';
import Application from '../../app';
import config from '../../config/environment';
import registerPowerSelectHelpers from 'ember-power-select/test-support/helpers';
import registerCodeMirrorHelpers from 'nomad-ui/tests/helpers/codemirror';

registerPowerSelectHelpers();
registerCodeMirrorHelpers();

export default function startApp(attrs) {
  let attributes = merge({}, config.APP);
  attributes = merge(attributes, attrs); // use defaults, but you can override;

  return run(() => {
    let application = Application.create(attributes);
    application.setupForTesting();
    application.injectTestHelpers();
    return application;
  });
}
