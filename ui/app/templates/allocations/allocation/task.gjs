/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import Breadcrumb from 'nomad-ui/components/breadcrumb';

<template>
  <Breadcrumb
    @crumb={{hash
      title="Task"
      label=@model.name
      args=(array "allocations.allocation.task" @model.allocation @model.name)
    }}
  />
  {{outlet}}
</template>
