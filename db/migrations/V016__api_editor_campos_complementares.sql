-- Campos do VisionFlow sem origem disponível no schema ERP atual.

ALTER TABLE public.cliente
    ADD COLUMN data_ultima_visita date,
    ADD COLUMN anotacao_usuario text;

COMMENT ON COLUMN public.cliente.data_ultima_visita IS
    'Data da última visita informada/confirmada pelo usuário do VisionFlow.';
COMMENT ON COLUMN public.cliente.anotacao_usuario IS
    'Anotação mantida pelo usuário do VisionFlow, independente de anotacao_erp.';

ALTER TABLE public.pedido_item
    ADD COLUMN codigo_cliente varchar(50);

COMMENT ON COLUMN public.pedido_item.codigo_cliente IS
    'Código do produto usado pelo cliente, informado no item do pedido.';
