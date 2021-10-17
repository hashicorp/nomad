import 'core-js';
import Application from 'nomad-ui/app';
import config from 'nomad-ui/config/environment';
import { setApplication } from '@ember/test-helpers';
import { start } from 'ember-qunit';
import { useNativeEvents } from 'ember-cli-page-object/extend';

useNativeEvents();

setApplication(Application.create(config.APP));

start();
