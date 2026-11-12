import type { ComponentChildren } from 'preact';
import { Icon, type IconName } from './Icon';
import './Accordion.css';

interface Props {
  icon: IconName;
  title: string;
  /** One line shown under the title, so closed sections still tell you their state. */
  summary: string;
  defaultOpen?: boolean;
  children: ComponentChildren;
}

export function Accordion({ icon, title, summary, defaultOpen, children }: Props) {
  return (
    <details class="card acc" open={defaultOpen}>
      <summary>
        <span class="ico">
          <Icon name={icon} />
        </span>
        <span class="t">
          {title}
          <small>{summary}</small>
        </span>
      </summary>
      <div class="body">{children}</div>
    </details>
  );
}
