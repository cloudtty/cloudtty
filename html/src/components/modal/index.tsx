import { h, Component, ComponentChildren } from 'preact';

import './modal.scss';

interface Props {
    show: boolean;
    children: ComponentChildren;
    onClose: () => void;
}

export class Modal extends Component<Props> {
    constructor(props: Props) {
        super(props);
    }

    render({ show, children }: Props) {
        return (
            show && (
                <div className="modal">
                    <div className="modal-background" />
                    <div className="modal-content">
                        <div className="modal-close" onClick={() => this.props.onClose()}>x</div>
                        <div className="box">{children}</div>
                    </div>
                </div>
            )
        );
    }
}
