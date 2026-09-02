-- +goose Up
ALTER TABLE pickup_operation_students
    DROP CHECK chk_pickup_operation_students_status,
    ADD CONSTRAINT chk_pickup_operation_students_status CHECK (status IN ('planned', 'picked_up', 'self_arrived', 'parent_picked_up', 'leave', 'absent', 'arrived', 'not_arrived', 'left', 'midway_left', 'abnormal'));

-- +goose Down
ALTER TABLE pickup_operation_students
    DROP CHECK chk_pickup_operation_students_status,
    ADD CONSTRAINT chk_pickup_operation_students_status CHECK (status IN ('planned', 'picked_up', 'self_arrived', 'parent_picked_up', 'leave', 'absent'));
