package proxmox

import (
	"fmt"
	pxapi "github.com/Telmate/proxmox-api-go/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strconv"
	"strings"
	"time"
)

func resourceVmQemuSnapshotRollback() *schema.Resource {
	*pxapi.Debug = true
	return &schema.Resource{
		Create:        resourceVmQemuSnapshotRollbackCreate,
		Read:          resourceVmQemuSnapshotRollbackRead,
		Update:        resourceVmQemuSnapshotRollbackUpdate,
		Delete:        resourceVmQemuSnapshotRollbackDelete,
		CustomizeDiff: resourceVmQemuSnapshotRollbackDiff,
		Importer: &schema.ResourceImporter{
			State: resourceVmQemuRollbackImport,
		},

		Schema: map[string]*schema.Schema{
			"vm_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"snapshot": {
				Type:     schema.TypeString,
				Required: true,
			},
			"ssh_forward_ip": {
				Type:     schema.TypeString,
				Required: true,
			},
			"ssh_user": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssh_password": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssh_private_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
			"timestamp": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceVmQemuSnapshotRollbackDiff(d *schema.ResourceDiff, meta interface{}) error {
	old, new := d.GetChange("timestamp")
	rollbackTime, _ := time.Parse(time.RFC3339, old.(string))
	currentTime, _ := time.Parse(time.RFC3339, new.(string))

	if currentTime.Sub(rollbackTime).Seconds() < 60.0 {
		return fmt.Errorf("VM rollback failed: less than 1 minute from last run")
	}

	return nil
}

func resourceVmQemuSnapshotRollbackCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	client := pconf.Client

	vm_id := d.Get("vm_id").(int)
	snapshot := d.Get("snapshot").(string)

	vmr := pxapi.NewVmRef(vm_id)

	log.Print("[DEBUG] rollback VM")
	exitStatus, err := client.RollbackQemuVm(vmr, snapshot)
	if err != nil {
		pmParallelEnd(pconf)
		return err
	}
	if exitStatus != "OK" {
		pmParallelEnd(pconf)
		return fmt.Errorf("VM rollback failed: %s", exitStatus)
	}

	// give sometime to proxmox to catchup
	time.Sleep(1 * time.Second)

	log.Print("[DEBUG] starting VM")
	_, err = client.StartVm(vmr)
	if err != nil {
		pmParallelEnd(pconf)
		return err
	}

	// Done with proxmox API, end parallel and do the SSH things
	pmParallelEnd(pconf)

	d.SetConnInfo(map[string]string{
		"type":        "ssh",
		"host":        d.Get("ssh_forward_ip").(string),
		"user":        d.Get("ssh_user").(string),
		"password":    d.Get("ssh_password").(string),
		"private_key": d.Get("ssh_private_key").(string),
		"pm_api_url":  client.ApiUrl,
		"pm_user":     client.Username,
		"pm_password": client.Password,
	})

	d.SetId(strconv.Itoa(vm_id))

	return nil
}

func resourceVmQemuSnapshotRollbackUpdate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceVmQemuSnapshotRollbackRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceVmQemuRollbackImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// TODO: research proper import
	err := resourceVmQemuRead(d, meta)
	return []*schema.ResourceData{d}, err
}

func resourceVmQemuSnapshotRollbackDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
