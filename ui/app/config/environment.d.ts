/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Type declarations for
 *    import config from 'nomad-ui/config/environment'
 */
declare const config: {
  environment: string;
  modulePrefix: string;
  podModulePrefix: string;
  locationType: 'history' | 'hash' | 'none';
  rootURL: string;
  APP: Record<string, unknown>;
  'ember-cli-mirage'?: {
    enabled: boolean;
    excludeFilesFromBuild: boolean;
  };
};

export default config;
