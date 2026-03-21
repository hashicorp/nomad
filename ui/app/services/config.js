/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { get } from '@ember/object';
import config from '../config/environment';

export default class ConfigService extends Service {
  unknownProperty(path) {
    return get(config, path);
  }

  get isDev() {
    return this.environment === 'development';
  }

  get isProd() {
    return this.environment === 'production';
  }

  get isTest() {
    return this.environment === 'test';
  }
}
