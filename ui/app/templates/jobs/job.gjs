/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import didUpdateHelper from 'ember-render-helpers/helpers/did-update-helper';

<template>
  {{didUpdateHelper
    @controller.notFoundJobHandler
    @controller.watchers.job.isError
  }}
  <Breadcrumb @crumb={{hash type="job" job=@model}} />{{outlet}}
</template>
