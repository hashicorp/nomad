import 'core-js';
import Application from '../app';
import config from '../config/environment';
import { setApplication } from '@ember/test-helpers';
import { start } from 'ember-qunit';
import { useNativeEvents } from 'ember-cli-page-object/extend';
import sinon from 'sinon';

if (config.percy.enabled) {
  sinon.useFakeTimers({ shouldAdvanceTime: true });
}

useNativeEvents();

setApplication(Application.create(config.APP));

start();
