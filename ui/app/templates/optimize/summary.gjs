/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Breadcrumb from 'nomad-ui/components/breadcrumb';
import DasRecommendationCard from 'nomad-ui/components/das/recommendation-card';

<template>
  <Breadcrumb @crumb={{@controller.breadcrumb}} />
  <DasRecommendationCard
    @summary={{@model}}
    @proceed={{@controller.optimizeController.proceed}}
  />
</template>
