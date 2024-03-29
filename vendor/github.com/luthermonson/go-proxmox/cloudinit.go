//go:build fs
// +build fs

package proxmox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

// CloudInit takes four yaml docs as a string and make an ISO, upload it to the data store as <vmid>-user-data.iso and will
// mount it as a CD-ROM to be used with nocloud cloud-init. This is NOT how proxmox expects a user to do cloud-init
// which can be found here: https://pve.proxmox.com/wiki/Cloud-Init_Support#:~:text=and%20meta.-,Cloud%2DInit%20specific%20Options,-cicustom%3A%20%5Bmeta
// If you want to use the proxmox implementation you'll need to use the cloudinit APIs https://pve.proxmox.com/pve-docs/api-viewer/index.html#/nodes/{node}/qemu/{vmid}/cloudinit
func (v *VirtualMachine) CloudInit(ctx context.Context, device, userdata, metadata, vendordata, networkconfig string) error {
	isoName := fmt.Sprintf(UserDataISOFormat, v.VMID)
	// create userdata iso file on the local fs
	iso, err := makeCloudInitISO(isoName, userdata, metadata, vendordata, networkconfig)
	if err != nil {
		return err
	}

	defer func() {
		// _ = os.Remove(iso.Name())
	}()

	node, err := v.client.Node(ctx, v.Node)
	if err != nil {
		return err
	}

	storage, err := node.StorageISO(ctx)
	if err != nil {
		return err
	}

	task, err := storage.Upload("iso", iso.Name())
	if err != nil {
		return err
	}

	// iso should only be < 5mb so wait for it and then mount it
	if err := task.WaitFor(ctx, 5); err != nil {
		return err
	}

	_, err = v.AddTag(ctx, MakeTag(TagCloudInit))
	if err != nil && !IsErrNoop(err) {
		return err
	}

	task, err = v.Config(ctx, VirtualMachineOption{
		Name:  device,
		Value: fmt.Sprintf("%s:iso/%s,media=cdrom", storage.Name, isoName),
	}, VirtualMachineOption{
		Name:  "boot",
		Value: fmt.Sprintf("%s;%s", v.VirtualMachineConfig.Boot, device),
	})

	if err != nil {
		return err
	}

	return task.WaitFor(ctx, 2)
}

func makeCloudInitISO(filename, userdata, metadata, vendordata, networkconfig string) (iso *os.File, err error) {
	iso, err = os.Create(filepath.Join(os.TempDir(), filename))
	if err != nil {
		return nil, err
	}

	defer func() {
		err = iso.Close()
	}()

	fs, err := iso9660.Create(iso, 0, 0, blockSize, "")
	if err != nil {
		return nil, err
	}

	if err := fs.Mkdir("/"); err != nil {
		return nil, err
	}

	cifiles := map[string]string{
		"user-data": userdata,
		"meta-data": metadata,
	}
	if vendordata != "" {
		cifiles["vendor-data"] = vendordata
	}
	if networkconfig != "" {
		cifiles["network-config"] = networkconfig
	}

	for filename, content := range cifiles {
		rw, err := fs.OpenFile("/"+filename, os.O_CREATE|os.O_RDWR)
		if err != nil {
			return nil, err
		}

		if _, err := rw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err = fs.Finalize(iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: volumeIdentifier,
	}); err != nil {
		return nil, err
	}

	return
}
