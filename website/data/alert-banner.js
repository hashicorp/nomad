export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  url:
    'https://www.hashicorp.com/blog/announcing-general-availability-of-hashicorp-nomad-0-12',
  tag: 'ANNOUNCING',
  text:
    'Nomad 0.12 is now generally available, which includes 15+ new features and our breakthrough Multi-Cluster Deployment. Learn more!',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: null,
}
