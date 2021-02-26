import s from './style.module.css'

export default function Callout({ children }) {
  return <div className={s.callout}>{children}</div>
}
