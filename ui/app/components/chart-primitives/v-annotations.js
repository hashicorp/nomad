import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { action } from '@ember/object';

const iconFor = {
  error: 'cancel-circle-fill',
  info: 'info-circle-fill',
};

const iconClassFor = {
  error: 'is-danger',
  info: '',
};

// TODO: This is what styleStringProperty looks like in the pure decorator world
function styleString(target, name, descriptor) {
  if (!descriptor.get) throw new Error('styleString only works on getters');
  const orig = descriptor.get;
  descriptor.get = function() {
    const styles = orig.apply(this);

    let str = '';

    if (styles) {
      str = Object.keys(styles)
        .reduce(function(arr, key) {
          const val = styles[key];
          arr.push(key + ':' + (typeof val === 'number' ? val.toFixed(2) + 'px' : val));
          return arr;
        }, [])
        .join(';');
    }

    return htmlSafe(str);
  };
  return descriptor;
}

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
    return sortedAnnotations.map(annotation => {
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
      };
    });
  }

  @action
  selectAnnotation(annotation) {
    if (this.args.annotationClick) this.args.annotationClick(annotation);
  }
}
