import { bind } from 'decko';
import { h, Component } from 'preact';
import { saveAs } from 'file-saver';
import { IDisposable, ITerminalAddon, Terminal } from 'xterm';
import * as Zmodem from 'zmodem.js/src/zmodem_browser';

import { Modal } from '../modal';

export interface FlowControl {
    limit: number;
    highWater: number;
    lowWater: number;

    pause: () => void;
    resume: () => void;
}

interface Props {
    sender: (data: ArrayLike<number>) => void;
    control: FlowControl;
    modalDownload: boolean;
    modalUpload: boolean;
    change: (v: string) => void;
    hideDownload: (v: boolean) => void;
    hideUpload: (v: boolean) => void;
}

interface State {
    value: string;
}

export class ZmodemAddon extends Component<Props, State> implements ITerminalAddon {
    private terminal: Terminal | undefined;
    private keyDispose: IDisposable | undefined;
    private sentry: Zmodem.Sentry;
    private session: Zmodem.Session;

    private written = 0;
    private pending = 0;

    constructor(props: Props) {
        super(props);

        this.zmodemInit();
    }

    render() {
        return (
            <div>
                <Modal show={this.props.modalUpload} onClose={() => {this.props.hideUpload(false)}}>
                    <label class="file-label">
                        <input onChange={this.sendFile} class="file-input" type="file" multiple />
                        <span class="file-cta">选择文件</span>
                    </label>
                </Modal>
                <Modal show={this.props.modalDownload} onClose={() => {this.props.hideDownload(false)}}>
                    <form onSubmit={this.onSubmit}>
                        <label class="file-label">
                            文件路径: <input class="path-input" type="text" value={this.state.value} onInput={this.onInput}/>
                        </label>
                        <button class="file-button" type="submit">下载文件</button>
                    </form>
                </Modal>
            </div>
        );
    }

    state = { value: '' };

    onInput = ev => {
        this.setState({ value: ev.target.value });
    }
  
    onSubmit = ev => {
        ev.preventDefault();
  
        this.props.change(this.state.value);
        this.setState({ value: '' });
        this.props.hideDownload(false);
    }

    async componentWillUnmount() {
        this.setState({ value: '' });
    }

    activate(terminal: Terminal): void {
        this.terminal = terminal;
    }

    dispose(): void {}

    consume(data: ArrayBuffer) {
        const { sentry, handleError } = this;
        try {
            sentry.consume(data);
        } catch (e) {
            handleError(e, 'consume');
        }
    }

    @bind
    private handleError(e: Error, reason: string) {
        console.error(`[ttyd] zmodem ${reason}: `, e);
        this.zmodemReset();
    }

    @bind
    private zmodemInit() {
        this.session = null;
        this.sentry = new Zmodem.Sentry({
            to_terminal: (octets: ArrayBuffer) => this.zmodemWrite(octets),
            sender: (octets: ArrayLike<number>) => this.zmodemSend(octets),
            on_retract: () => this.zmodemReset(),
            on_detect: (detection: Zmodem.Detection) => this.zmodemDetect(detection),
        });
    }

    @bind
    private zmodemReset() {
        this.terminal.setOption('disableStdin', false);

        if (this.keyDispose) {
            this.keyDispose.dispose();
            this.keyDispose = null;
        }
        this.zmodemInit();

        this.terminal.focus();
    }

    @bind
    private zmodemWrite(data: ArrayBuffer): void {
        const { limit, highWater, lowWater, pause, resume } = this.props.control;
        const { terminal } = this;
        const rawData = new Uint8Array(data);

        this.written += rawData.length;
        if (this.written > limit) {
            terminal.write(rawData, () => {
                this.pending = Math.max(this.pending - 1, 0);
                if (this.pending < lowWater) {
                    resume();
                }
            });
            this.pending++;
            this.written = 0;
            if (this.pending > highWater) {
                pause();
            }
        } else {
            terminal.write(rawData);
        }
    }

    @bind
    private zmodemSend(data: ArrayLike<number>): void {
        this.props.sender(data);
    }

    @bind
    private zmodemDetect(detection: Zmodem.Detection): void {
        const { terminal, receiveFile, zmodemReset } = this;
        terminal.setOption('disableStdin', true);

        this.keyDispose = terminal.onKey(e => {
            const event = e.domEvent;
            if (event.ctrlKey && event.key === 'c') {
                detection.deny();
            }
        });

        this.session = detection.confirm();
        this.session.on('session_end', zmodemReset);

        if (this.session.type === 'send') {
            this.props.hideUpload(true);
        } else {
            receiveFile();
        }
    }

    @bind
    private sendFile(event: Event) {
        this.props.hideUpload(false);

        const { session, writeProgress, handleError } = this;
        const files: FileList = (event.target as HTMLInputElement).files;

        Zmodem.Browser.send_files(session, files, {
            on_progress: (_, offer: Zmodem.Offer) => writeProgress(offer),
        })
            .then(() => session.close())
            .catch(e => handleError(e, 'send'));
    }

    @bind
    private receiveFile() {
        const { session, writeProgress, handleError } = this;

        session.on('offer', (offer: Zmodem.Offer) => {
            const fileBuffer = [];
            offer.on('input', payload => {
                writeProgress(offer);
                fileBuffer.push(new Uint8Array(payload));
            });
            offer
                .accept()
                .then(() => {
                    const blob = new Blob(fileBuffer, { type: 'application/octet-stream' });
                    saveAs(blob, offer.get_details().name);
                })
                .catch(e => handleError(e, 'receive'));
        });

        session.start();
    }

    @bind
    private writeProgress(offer: Zmodem.Offer) {
        const { terminal, bytesHuman } = this;

        const file = offer.get_details();
        const name = file.name;
        const size = file.size;
        const offset = offer.get_offset();
        const percent = ((100 * offset) / size).toFixed(2);

        terminal.write(`${name} ${percent}% ${bytesHuman(offset, 2)}/${bytesHuman(size, 2)}\r`);
    }

    private bytesHuman(bytes: any, precision: number): string {
        if (!/^([-+])?|(\.\d+)(\d+(\.\d+)?|(\d+\.)|Infinity)$/.test(bytes)) {
            return '-';
        }
        if (bytes === 0) return '0';
        if (typeof precision === 'undefined') precision = 1;
        const units = ['bytes', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const num = Math.floor(Math.log(bytes) / Math.log(1024));
        const value = (bytes / Math.pow(1024, Math.floor(num))).toFixed(precision);
        return `${value} ${units[num]}`;
    }
}
