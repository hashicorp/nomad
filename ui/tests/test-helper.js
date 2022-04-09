import 'core-js';
import Application from 'nomad-ui/app';
import config from 'nomad-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import start from 'ember-exam/test-support/start';
import { setup } from 'qunit-dom';

setApplication(Application.create(config.APP));

setup(QUnit.assert);

start();
