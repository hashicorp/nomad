/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { equal } from '@ember/object/computed';
import Service from '@ember/service';
import { get } from '@ember/object';
import config from '../config/environment';

export default class ConfigService extends Service {
  unknownProperty(path) {
    return get(config, path);
  }

  @equal('environment', 'development') isDev;
  @equal('environment', 'production') isProd;
  @equal('environment', 'test') isTest;
}
