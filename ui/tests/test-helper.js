import 'core-js';
import Application from '../app';
import config from '../config/environment';
import { setApplication } from '@ember/test-helpers';
import { start } from 'ember-qunit';
import { useNativeEvents } from 'ember-cli-page-object/extend';
import QUnit from 'qunit';

QUnit.testStart = function(test) {
  var module = test.module ? test.module : '';
  console.log('#' + module + ' ' + test.name + ': started.');
};

QUnit.moduleStart(function(details) {
  console.log('Now running: ', details.name);
});

useNativeEvents();

setApplication(Application.create(config.APP));

start();
