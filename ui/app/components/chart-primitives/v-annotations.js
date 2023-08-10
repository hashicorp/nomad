/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { action, get } from '@ember/object';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';

const iconFor = {
  error: 'cancel-circle-fill',
  info: 'info-circle-fill',
};

const iconClassFor = {
  error: 'is-danger',
  info: '',
};

export default class ChartPrimitiveVAnnotations extends Component {
  @styleString
  get chartAnnotationsStyle() {
    return {
      height: this.args.height,
    };
  }

  get processed() {
    const { scale, prop, annotations, timeseries, format } = this.args;

    if (!annotations || !annotations.length) return null;

    let sortedAnnotations = annotations.sortBy(prop);
    if (timeseries) {
      sortedAnnotations = sortedAnnotations.reverse();
    }

    let prevX = 0;
    let prevHigh = false;
    return sortedAnnotations.map((annotation) => {
      const x = scale(annotation[prop]);
      if (prevX && !prevHigh && Math.abs(x - prevX) < 30) {
        prevHigh = true;
      } else if (prevHigh) {
        prevHigh = false;
      }
      const y = prevHigh ? -15 : 0;
      const formattedX = format(timeseries)(annotation[prop]);

      prevX = x;
      return {
        annotation,
        style: htmlSafe(`transform:translate(${x}px,${y}px)`),
        icon: iconFor[annotation.type],
        iconClass: iconClassFor[annotation.type],
        staggerClass: prevHigh ? 'is-staggered' : '',
        label: `${annotation.type} event at ${formattedX}`,
        isActive: this.annotationIsActive(annotation),
      };
    });
  }

  annotationIsActive(annotation) {
    const { key, activeAnnotation } = this.args;
    if (!activeAnnotation) return false;

    if (key) return get(annotation, key) === get(activeAnnotation, key);
    return annotation === activeAnnotation;
  }

  @action
  selectAnnotation(annotation) {
    if (this.args.annotationClick) this.args.annotationClick(annotation);
  }
}
