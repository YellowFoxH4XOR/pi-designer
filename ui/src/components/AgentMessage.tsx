import { Icon } from './Icon';
import { QuestionForm } from './QuestionForm';
import { RichText } from './RichText';
import { parseAssistantText, type ParsedFormAnswers } from './protocol';

/** One assistant turn: design notes, the answer, forms, and next steps. */
export function AgentMessage({ text, live = false, interactive = false, answered, onSend }: {
  text: string;
  live?: boolean;
  interactive?: boolean;
  answered?: Map<string, ParsedFormAnswers>;
  onSend?: (text: string) => boolean | void | Promise<boolean | void>;
}) {
  const parsed = parseAssistantText(text);
  const notesOpen = live;
  return <div class="agent-message">
    {parsed.narration && <details class="design-notes" open={notesOpen}>
      <summary><Icon name="eye" size={14}/>Design notes<span class="design-notes-hint">{live ? 'Working…' : 'How π got there'}</span></summary>
      <div class="design-notes-body"><RichText text={parsed.narration}/></div>
    </details>}

    {parsed.answer && <RichText text={parsed.answer}/>}

    {parsed.pendingForm && (live
      ? <p class="form-preparing"><span class="working-indicator"><i/><i/><i/></span>Preparing questions…</p>
      : <p class="form-broken">π sent questions that couldn’t be displayed. Ask it to resend them.</p>)}

    {parsed.forms.map((form) => {
      if (form.invalid) {
        return <p class="form-broken" key={form.id}>π sent questions that couldn’t be displayed. Ask it to resend them.</p>;
      }
      const submitted = answered?.get(form.id);
      if (submitted) {
        return <details class="form-collapsed form-answered" key={form.id}>
          <summary><Icon name="check" size={14}/>{form.title || 'Questions'}<span>Answered</span></summary>
          {submitted.entries.length
            ? <dl class="form-answer-list">{submitted.entries.map((entry, index) => <div key={index}><dt>{entry.label}</dt><dd>{entry.display}</dd></div>)}</dl>
            : <p class="form-answer-empty">Skipped.</p>}
        </details>;
      }
      if (interactive) {
        return <QuestionForm key={form.id} form={form} onSubmit={onSend ?? (() => {})}/>;
      }
      return <details class="form-collapsed form-unanswered" key={form.id}>
        <summary><Icon name="skills" size={14}/>{form.title || 'Questions'}<span>{live ? 'Waiting for the turn to finish' : 'Unanswered'}</span></summary>
        <ul class="form-question-list">{form.questions.map((question) => <li key={question.id}>{question.label}</li>)}</ul>
      </details>;
    })}

    {interactive && parsed.next.length > 0 && <div class="next-steps" aria-label="Suggested replies">
      {parsed.next.map((sentence) => <button key={sentence} class="next-step" onClick={() => void onSend?.(sentence)}>{sentence}<Icon name="arrow" size={13}/></button>)}
    </div>}
  </div>;
}
