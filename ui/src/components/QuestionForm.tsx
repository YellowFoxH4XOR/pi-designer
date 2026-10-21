import { useRef, useState } from 'preact/hooks';
import { Icon } from './Icon';
import {
  OTHER_VALUE, formatFormAnswers, isAnswered, resolveDefault,
  type FormAnswers, type FormValue, type Question, type QuestionFormData,
} from './protocol';

let seq = 0;

export function QuestionForm({ form, disabled = false, onSubmit }: {
  form: QuestionFormData;
  disabled?: boolean;
  onSubmit: (text: string) => boolean | void | Promise<boolean | void>;
}) {
  const [uid] = useState(() => `qf${++seq}`);
  const [answers, setAnswers] = useState<FormAnswers>(() => {
    const initial: FormAnswers = {};
    for (const question of form.questions) initial[question.id] = resolveDefault(question);
    return initial;
  });
  const [custom, setCustom] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [sent, setSent] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const locked = disabled || sent;

  function clearError(id: string) {
    setErrors((all) => {
      if (!all[id]) return all;
      const next = { ...all };
      delete next[id];
      return next;
    });
  }

  function update(id: string, value: FormValue) {
    setAnswers((all) => ({ ...all, [id]: value }));
    clearError(id);
  }

  function effective(question: Question): FormValue {
    const raw = answers[question.id];
    if (question.type === 'checkbox') {
      const list = Array.isArray(raw) ? raw.map(String) : [];
      const rest = list.filter((value) => value !== OTHER_VALUE);
      const other = custom[question.id]?.trim();
      return list.includes(OTHER_VALUE) && other ? [...rest, other] : rest;
    }
    if ((question.type === 'radio' || question.type === 'select') && raw === OTHER_VALUE) {
      return custom[question.id]?.trim() ?? '';
    }
    return raw;
  }

  async function dispatch(payload: string) {
    if (locked) return;
    setSent(true);
    const result = await Promise.resolve(onSubmit(payload));
    if (result === false) setSent(false);
  }

  function submit(event: Event) {
    event.preventDefault();
    if (locked) return;
    const problems: Record<string, string> = {};
    for (const question of form.questions) {
      if (question.required && !isAnswered(question, effective(question))) {
        problems[question.id] = 'Choose an answer to continue.';
      }
    }
    if (Object.keys(problems).length) {
      setErrors(problems);
      const first = form.questions.find((question) => problems[question.id]);
      requestAnimationFrame(() => {
        if (!first) return;
        const field = [...(formRef.current?.querySelectorAll<HTMLElement>('fieldset') ?? [])]
          .find((element) => element.dataset.question === first.id);
        field?.querySelector<HTMLElement>('input, select, textarea, button')?.focus();
      });
      return;
    }
    const payload: FormAnswers = {};
    for (const question of form.questions) payload[question.id] = effective(question);
    void dispatch(formatFormAnswers(form, payload));
  }

  function skip() {
    void dispatch(formatFormAnswers(form, {}));
  }

  function control(question: Question, index: number, error?: string) {
    const id = `${uid}-${index}`;
    const errorId = error ? `${id}-error` : undefined;
    const value = answers[question.id];
    const common = {
      disabled: locked,
      'aria-invalid': error ? true : undefined,
      'aria-describedby': errorId,
      'aria-required': question.required || undefined,
    };
    const customEnabled = question.allowCustom !== false;
    const isOther = value === OTHER_VALUE;

    if (question.type === 'radio') {
      return <>
        <div class="question-options">
          {(question.options ?? []).map((option) => <label class="option-row" key={option.value}>
            <input type="radio" name={id} value={option.value} checked={value === option.value} {...common} onChange={() => update(question.id, option.value)}/>
            <span class="option-copy"><span>{option.label}</span>{option.description && <small>{option.description}</small>}</span>
          </label>)}
          {customEnabled && <label class="option-row">
            <input type="radio" name={id} value={OTHER_VALUE} checked={isOther} {...common} onChange={() => update(question.id, OTHER_VALUE)}/>
            <span class="option-copy"><span>Other…</span></span>
          </label>}
        </div>
        {customEnabled && isOther && <input id={`${id}-other`} class="other-input" type="text" aria-label={`Other answer for ${question.label}`} placeholder="Type your answer" value={custom[question.id] ?? ''} disabled={locked}
          onInput={(e) => { setCustom((all) => ({ ...all, [question.id]: e.currentTarget.value })); clearError(question.id); }}/>}
      </>;
    }

    if (question.type === 'checkbox') {
      const selected = Array.isArray(value) ? value.map(String) : [];
      const atLimit = question.maxSelections !== undefined && selected.length >= question.maxSelections;
      const toggle = (option: string, on: boolean) => {
        const without = selected.filter((entry) => entry !== option);
        if (on && question.maxSelections !== undefined && selected.length >= question.maxSelections) return;
        update(question.id, on ? [...without, option] : without);
      };
      return <>
        {question.maxSelections !== undefined && <p class="question-hint">Choose up to {question.maxSelections}.</p>}
        <div class="question-options">
          {(question.options ?? []).map((option) => {
            const checked = selected.includes(option.value);
            return <label class="option-row" key={option.value}>
              <input type="checkbox" value={option.value} checked={checked} {...common} disabled={locked || (atLimit && !checked)} onChange={(e) => toggle(option.value, e.currentTarget.checked)}/>
              <span class="option-copy"><span>{option.label}</span>{option.description && <small>{option.description}</small>}</span>
            </label>;
          })}
          {customEnabled && <label class="option-row">
            <input type="checkbox" value={OTHER_VALUE} checked={selected.includes(OTHER_VALUE)} {...common}
              disabled={locked || (atLimit && !selected.includes(OTHER_VALUE))}
              onChange={(e) => toggle(OTHER_VALUE, e.currentTarget.checked)}/>
            <span class="option-copy"><span>Other…</span></span>
          </label>}
        </div>
        {customEnabled && selected.includes(OTHER_VALUE) && <input class="other-input" type="text" aria-label={`Other answer for ${question.label}`} placeholder="Type your answer" value={custom[question.id] ?? ''} disabled={locked}
          onInput={(e) => { setCustom((all) => ({ ...all, [question.id]: e.currentTarget.value })); clearError(question.id); }}/>}
      </>;
    }

    if (question.type === 'select') {
      return <>
        <select id={id} value={typeof value === 'string' ? value : ''} {...common} onChange={(e) => update(question.id, e.currentTarget.value)}>
          <option value="">Select…</option>
          {(question.options ?? []).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          {customEnabled && <option value={OTHER_VALUE}>Other…</option>}
        </select>
        {customEnabled && isOther && <input class="other-input" type="text" aria-label={`Other answer for ${question.label}`} placeholder="Type your answer" value={custom[question.id] ?? ''} disabled={locked}
          onInput={(e) => { setCustom((all) => ({ ...all, [question.id]: e.currentTarget.value })); clearError(question.id); }}/>}
      </>;
    }

    if (question.type === 'textarea') {
      return <textarea id={id} rows={3} placeholder={question.placeholder} value={typeof value === 'string' ? value : ''} {...common}
        onInput={(e) => update(question.id, e.currentTarget.value)}/>;
    }

    if (question.type === 'switch') {
      return <label class="switch-row">
        <input id={id} type="checkbox" role="switch" checked={value === true} {...common} onChange={(e) => update(question.id, e.currentTarget.checked)}/>
        <span class="switch-track" aria-hidden="true"><span/></span>
        <span class="switch-label">{value === true ? 'Yes' : 'No'}</span>
      </label>;
    }

    if (question.type === 'range') {
      const current = typeof value === 'number' ? value : Number(value) || question.min || 0;
      return <div class="range-row">
        <input id={id} type="range" min={question.min} max={question.max} step={question.step} value={current} {...common}
          onInput={(e) => update(question.id, Number(e.currentTarget.value))}/>
        <output for={id}>{current}</output>
      </div>;
    }

    const inputType = question.type === 'number' || question.type === 'color' || question.type === 'url' || question.type === 'email' || question.type === 'date'
      ? question.type : 'text';
    const inputValue = inputType === 'number'
      ? (typeof value === 'number' || typeof value === 'string' ? value : '')
      : inputType === 'color'
        ? (typeof value === 'string' && /^#[0-9a-f]{6}$/i.test(value) ? value : '#ffffff')
        : typeof value === 'string' ? value : String(value ?? '');
    return <input id={id} type={inputType} placeholder={question.placeholder} value={inputValue} {...common}
      min={inputType === 'number' ? question.min : undefined}
      max={inputType === 'number' ? question.max : undefined}
      step={inputType === 'number' ? question.step : undefined}
      onInput={(e) => update(question.id, inputType === 'number' && e.currentTarget.value !== '' ? Number(e.currentTarget.value) : e.currentTarget.value)}/>;
  }

  return <form ref={formRef} class="question-form" onSubmit={submit} noValidate aria-label={form.title || 'Questions from Pi'}>
    <div class="question-form-head">
      <span class="question-form-kicker"><Icon name="skills" size={15}/>A few quick questions</span>
      {form.title && <h3 class="question-form-title">{form.title}</h3>}
    </div>
    {form.questions.map((question, index) => {
      const error = errors[question.id];
      return <fieldset key={question.id} class={`question-field ${error ? 'has-error' : ''}`} data-question={question.id}>
        <legend>{question.label}{question.required && <span class="required-mark" aria-hidden="true"> *</span>}</legend>
        {control(question, index, error)}
        {error && <p class="question-error" id={`${uid}-${index}-error`} role="alert">{error}</p>}
      </fieldset>;
    })}
    <div class="question-form-actions">
      <button type="submit" class="button primary" disabled={locked}>{sent ? 'Sent' : form.submitLabel || 'Continue'}</button>
      <button type="button" class="text-button" disabled={locked} onClick={skip}>Skip</button>
    </div>
  </form>;
}
